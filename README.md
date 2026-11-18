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
│       └── assets/          # Coloque seus arquivos .mp3 aqui antes de compilar
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

1. Coloque seus arquivos `.mp3` (`success.mp3`, `error.mp3`, `faaah.mp3`, `mac-quack.mp3`, `anime-wow-sound-effect.mp3`, `bolsonaro-pegaram-meu-telefone.mp3`, `notification.mp3`) dentro de `cmd/fuoda.sh/assets/` ou em `~/.local/share/fuoda.sh/sounds/`.
2. Sincronize as dependências e instale (funciona no Linux e macOS):

```bash
go mod tidy
make install
```

## Uso

### Efeitos Sonoros no Terminal
```bash
fuoda.sh wow
fuoda.sh --sync --volume 80 warning
fuoda.sh list
```

### Servidor SSH com Handshake Sonoro
1. Gere sua sequência secreta de teclas (toca `success` ao salvar ou `warning` ao cancelar):
```bash
fuoda.sh ssh --keygen
```

2. Inicie o servidor SSH:
```bash
fuoda.sh ssh --addr :2222
```

3. Conecte via SSH:
```bash
ssh localhost -p 2222
```

### Eventos Sonoros Disparados no SSH
| Situação | Evento Disparado | Áudio Padrão |
| :--- | :--- | :--- |
| Servidor iniciado / nova conexão recebida | `notification` | `notification.mp3` |
| Sequência de handshake correta | `success` | `success.mp3` |
| Sequência errada ou cancelada (`Esc`/`Ctrl+C`) | `error` | `error.mp3` |
| Tempo esgotado (10s) no handshake | `faaah` | `faaah.mp3` |
| Conexão sem PTY / keygen cancelado | `warning` | `mac-quack.mp3` |


### Caso de warning por


ssh-keygen -f '/home/lodash/.ssh/known_hosts' -R '[localhost]:2222'
ssh localhost -p 2222