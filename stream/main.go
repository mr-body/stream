package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"golang.org/x/crypto/ssh"
)

type SSHConfig struct {
	User       string `json:"user"`
	Password   string `json:"password,omitempty"`
	PrivateKey string `json:"privateKey,omitempty"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
}

type ControlMsg struct {
	Type string `json:"type"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

type SSHConnection struct {
	sshClient  *ssh.Client
	sshSession *ssh.Session
	ws         *websocket.Conn
	inputChan  chan []byte
	mu         sync.Mutex
	connected  bool
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func NewSSHConnection(cfg SSHConfig) (*SSHConnection, error) {
	var auth ssh.AuthMethod
	if cfg.Password != "" {
		auth = ssh.Password(cfg.Password)
	} else if cfg.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(cfg.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("erro ao parsear chave privada: %w", err)
		}
		auth = ssh.PublicKeys(signer)
	} else {
		return nil, fmt.Errorf("nenhuma credencial fornecida")
	}

	sshCfg := &ssh.ClientConfig{
		User:            cfg.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), sshCfg)
	if err != nil {
		return nil, fmt.Errorf("erro dial ssh: %w", err)
	}

	return &SSHConnection{
		sshClient: client,
		connected: true,
		inputChan: make(chan []byte, 2048),
	}, nil
}

func (c *SSHConnection) StartShell() error {
	session, err := c.sshClient.NewSession()
	if err != nil {
		return fmt.Errorf("erro criar sessao: %w", err)
	}
	c.sshSession = session

	stdin, err := session.StdinPipe()
	if err != nil {
		return fmt.Errorf("erro stdin pipe: %w", err)
	}
	stdout, err := session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("erro stdout pipe: %w", err)
	}
	stderr, err := session.StderrPipe()
	if err != nil {
		return fmt.Errorf("erro stderr pipe: %w", err)
	}

	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	// Tamanho inicial (pode mudar por resize)
	if err := session.RequestPty("xterm-256color", 40, 80, modes); err != nil {
		return fmt.Errorf("erro RequestPty: %w", err)
	}

	if err := session.Shell(); err != nil {
		return fmt.Errorf("erro iniciar shell: %w", err)
	}

	// Leitura de saída (stdout + stderr)
	go c.pipeToWS(stdout)
	go c.pipeToWS(stderr)

	// Escrita para stdin a partir do canal
	go func() {
		defer stdin.Close()
		for c.connected {
			b, ok := <-c.inputChan
			if !ok {
				return
			}
			if len(b) == 0 {
				continue
			}
			_, _ = stdin.Write(b) // ignora erro de escrita (sessão pode fechar)
		}
	}()

	return nil
}

func (c *SSHConnection) pipeToWS(r io.Reader) {
	buf := make([]byte, 2048)
	for c.connected {
		n, err := r.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Println("erro leitura SSH:", err)
			}
			break
		}
		if n > 0 && c.ws != nil {
			// envia binário
			_ = c.ws.WriteMessage(websocket.BinaryMessage, buf[:n])
		}
	}
}

func (c *SSHConnection) WriteToSSH(b []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected {
		return fmt.Errorf("conexão fechada")
	}
	// Copia para evitar problemas se a slice for reutilizada pelo caller
	buf := make([]byte, len(b))
	copy(buf, b)
	select {
	case c.inputChan <- buf:
	default:
		// canal cheio -> descarta para evitar bloqueio
		log.Println("inputChan cheio, descartando dados")
	}
	return nil
}

func (c *SSHConnection) Resize(cols, rows int) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.sshSession == nil {
		return fmt.Errorf("sessao nao iniciada")
	}
	return c.sshSession.WindowChange(rows, cols)
}

func (c *SSHConnection) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.connected {
		return
	}
	c.connected = false
	close(c.inputChan)
	if c.sshSession != nil {
		c.sshSession.Close()
	}
	if c.sshClient != nil {
		c.sshClient.Close()
	}
}

// Handler WebSocket
func handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	defer ws.Close()
	log.Println("cliente WS conectado")

	// primeiro esperamos receber um TextMessage JSON com as credenciais/config
	msgType, msg, err := ws.ReadMessage()
	if err != nil {
		log.Println("erro ler primeira mensagem:", err)
		return
	}
	if msgType != websocket.TextMessage {
		ws.WriteMessage(websocket.TextMessage, []byte("esperado TextMessage com config"))
		return
	}

	var cfg SSHConfig
	if err := json.Unmarshal(msg, &cfg); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("erro JSON config: "+err.Error()))
		return
	}

	sshConn, err := NewSSHConnection(cfg)
	if err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("erro conectar SSH: "+err.Error()))
		return
	}
	sshConn.ws = ws
	defer sshConn.Close()

	if err := sshConn.StartShell(); err != nil {
		ws.WriteMessage(websocket.TextMessage, []byte("erro iniciar shell: "+err.Error()))
		return
	}

	// sinaliza prontidão
	ws.WriteMessage(websocket.TextMessage, []byte("ssh-ready"))

	// loop principal: mensagens binárias -> para ssh; textos -> controle (resize)
	for {
		msgType, msg, err := ws.ReadMessage()
		if err != nil {
			log.Println("ws read error:", err)
			break
		}
		switch msgType {
		case websocket.BinaryMessage:
			// dados brutos do terminal (do browser) -> SSH stdin
			if err := sshConn.WriteToSSH(msg); err != nil {
				log.Println("erro WriteToSSH:", err)
				break
			}
		case websocket.TextMessage:
			// controle (JSON)
			var ctl ControlMsg
			if err := json.Unmarshal(msg, &ctl); err != nil {
				log.Println("json controle invalido:", err)
				continue
			}
			if ctl.Type == "resize" {
				if err := sshConn.Resize(ctl.Cols, ctl.Rows); err != nil {
					log.Println("erro resize:", err)
				}
			} else {
				// outros tipos de controle podem ser tratados aqui
				log.Println("controle:", ctl.Type)
			}
		default:
			// ignorar ping/pong/continuation etc (o gorilla trata ping/pong automaticamente)
		}
	}

	log.Println("cliente desconectado")
}

func main() {
	http.HandleFunc("/ws", handleWS)
	// serve um diretório static (coloque o HTML do cliente em ./static/index.html)
	http.Handle("/", http.FileServer(http.Dir("./static")))

	addr := ":3004"
	log.Printf("rodando em %s (ws endpoint ws://localhost%s/ws)\n", addr, addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
