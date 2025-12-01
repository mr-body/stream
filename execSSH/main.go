package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"google.golang.org/grpc/credentials"
	"golang.org/x/crypto/ssh"
	"google.golang.org/grpc"
	sshpb "execSHH/protos/sshgrpc/sshpb"
)

type server struct {
	sshpb.UnimplementedSSHServer
}

func (s *server) ExecCommand(ctx context.Context, req *sshpb.SSHRequest) (*sshpb.SSHResponse, error) {

	// Criar auth method
	var auth ssh.AuthMethod

	if req.PrivateKey != "" {
		signer, err := ssh.ParsePrivateKey([]byte(req.PrivateKey))
		if err != nil {
			return nil, fmt.Errorf("erro ao ler a chave: %w", err)
		}
		auth = ssh.PublicKeys(signer)
	} else {
		auth = ssh.Password(req.Password)
	}

	config := &ssh.ClientConfig{
		User:            req.User,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	// Conectar ao host
	addr := fmt.Sprintf("%s:%d", req.Host, req.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, fmt.Errorf("erro ao conectar: %w", err)
	}
	defer client.Close()

	// Criar sessão
	session, err := client.NewSession()
	if err != nil {
		return nil, fmt.Errorf("erro ao criar sessão: %w", err)
	}
	defer session.Close()

	var stdout, stderr bytes.Buffer
	session.Stdout = &stdout
	session.Stderr = &stderr

	// Executar comando
	err = session.Run(req.Command)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			exitCode = -1
		}
	}

	return &sshpb.SSHResponse{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: int32(exitCode),
	}, nil
}

func main() {
    creds, err := credentials.NewServerTLSFromFile("certificate/server.crt", "certificate/server.key")
    if err != nil {
        log.Fatalf("falha ao carregar TLS: %v", err)
    }

    lis, err := net.Listen("tcp", ":50051")
    if err != nil {
        log.Fatal(err)
    }

    grpcServer := grpc.NewServer(grpc.Creds(creds))
    sshpb.RegisterSSHServer(grpcServer, &server{})

    log.Println("Servidor gRPC TLS rodando em 50051...")
    grpcServer.Serve(lis)
}
