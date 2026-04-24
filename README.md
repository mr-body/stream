# Stream

Uma solução para **streaming de terminal via SSH** com duas abordagens:

1. **WebSocket**: sessão PTY interativa (terminal “ao vivo”) no navegador.
2. **gRPC**: API para executar comandos via SSH.

## Visão geral (arquitetura)

```
            ┌──────────────┐
            │   Browser    │
            └──────┬───────┘
               WebSocket
                   ↓
        ┌──────────────────────────┐
        │   Go WebSocket Server    │
        │  (PTY SSH session live)  │
        └──────────┬───────────────┘
                  SSH
                    ��
               ┌──────────────┐
               │  Remote VPS  │
               └──────────────┘n

        ┌──────────────────────────┐
        │   gRPC SSH Server        │
        │ (exec command API)       │
        └──────────┬───────────────┘
                   SSH
                    ↓
               ┌──────────────┐
               │  Remote VPS  │
               └──────────────┘
```

## Componentes

- **Go WebSocket Server**: mantém uma sessão PTY via SSH e encaminha entrada/saída em tempo real para o browser.
- **gRPC SSH Server**: expõe endpoints gRPC para executar comandos via SSH.

## Como começar

> Nota: como o repositório pode evoluir, ajuste os caminhos/nomes conforme a estrutura atual.

1. Configure o acesso SSH ao host remoto (VPS) e garanta que o servidor onde você roda este projeto consegue conectar.
2. Inicie o(s) serviço(s) (WebSocket e/ou gRPC).
3. Abra o cliente no browser (para WebSocket) ou consuma a API gRPC (para exec de comandos).

## Segurança

- Evite embutir credenciais no código.
- Prefira **chaves SSH** e variáveis de ambiente.
- Restrinja rede/ports e aplique autenticação/autorização no lado do servidor.

## Contribuindo

Pull requests e issues são bem-vindos.

## Licença

Defina a licença do projeto (ex.: MIT) ou adicione um arquivo `LICENSE`.
