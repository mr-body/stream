# stream
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
                        ↓
               ┌──────────────┐
               │  Remote VPS  │
               └──────────────┘


        ┌──────────────────────────┐
        │   gRPC SSH Server        │
        │ (exec command API)       │
        └──────────┬───────────────┘
                   SSH
                        ↓
               ┌──────────────┐
               │  Remote VPS  │
               └──────────────┘
