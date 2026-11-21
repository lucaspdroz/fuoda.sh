<p align="center">
  <img src="./assets/fuoda.sh.png" alt="fuoda.sh" width="200">
</p>

# fuoda.sh

CLI em Go que unifica efeitos sonoros para terminal/scripts com um servidor SSH interativo protegido por sequência de teclas (*handshake*) e feedback sonoro em tempo real.

## Estrutura

```text
fuoda.sh/
├── Makefile
├── go.mod
├── cmd/
│   └── fuoda.sh/
│       ├── main.go
│       ├── assets/          # Coloque seus arquivos .mp3 aqui antes de compilar
│       └── themes/
│         ├── Default/
│         └── [custom-theme]/ 
└── internal/
    ├── audio/
    │   ├── player.go        # Player MP3 via malgo (otimizado para Linux e Mac M1)
    │   └── timer.go
    ├── config/
    │   └── config.go        # Mapeamento de eventos e leitura de events.conf
    └── sshserver/
        └── server.go        # Servidor SSH (Wish + Bubbletea) com triggers sonoros
```

## Instalação

1. Coloque seus arquivos `.mp3` (`success.mp3`, `error.mp3`, `faaah.mp3`, `notification.mp3`) dentro de `cmd/fuoda.sh/assets/` ou em `~/.local/share/fuoda.sh/theme/[theme-name]`.
2. Sincronize as dependências e instale (funciona no Linux e macOS):

```bash
go mod tidy
make install
```

## Uso

### Efeitos Sonoros no Terminal
```bash
fuoda.sh error
fuoda.sh -volume 12 success  
fuoda.sh list
```

### Servidor SSH com Handshake Sonoro
1. Gere sua sequência secreta de teclas (toca `success` ao salvar ou `error` ao cancelar):
```bash
fuoda.sh ssh --keygen
```

2. Inicie o servidor SSH:
```bash
fuoda.sh ssh --serve :2222
```

3. Conecte via SSH:
```bash
ssh localhost -p 2222
```

### Eventos Sonoros Disparados no SSH
| Situação | Evento Disparado | Áudio Padrão |
| :--- | :--- | :--- |
| Sequência de handshake correta | `success` | `success.mp3` |
| Sequência errada ou cancelada (`Esc`/`Ctrl+C`) | `error` | `error.mp3` |
| Tempo esgotado (10s) no handshake | `error` | `error.mp3` |
| Conexão sem PTY / keygen cancelado | `error` | `error.mp3` |


### Caso de warning por falha de credencial

ssh-keygen -f '/home/lodash/.ssh/known_hosts' -R '[localhost]:2222'
ssh localhost -p 2222
## Testando no Windows (Docker)

Requer Docker Desktop com backend WSL2 (o áudio sai pelo PulseAudio do WSLg).

```bash
docker compose build
docker compose run --rm --service-ports fuoda   # abre um shell Linux em /app
make install                                    # dentro do container
fuoda.sh success
fuoda.sh ssh --serve :2222                       # conecte do Windows: ssh localhost -p 2222
```

#### Release (linux & Mac)

```bash
git tag v0.0.3c               
git push origin v0.0.3

./utils/release-macos.sh v0.0.3
```
