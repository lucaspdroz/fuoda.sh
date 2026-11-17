# fuoda.sh

CLI em Go para reproduzir efeitos sonoros no terminal e em scripts.

## Estrutura

```text
fuoda.sh/
├── Makefile
├── go.mod
├── go.sum
├── cmd/
│   └── fuoda.sh/
│       ├── main.go
│       └── assets/          # Coloque seus arquivos .mp3 aqui antes de compilar
└── internal/
    ├── audio/
    │   ├── player.go
    │   └── timer.go
    └── config/
        └── config.go
```

## Instalação

1. Copie seus arquivos `.mp3` para `cmd/fuoda.sh/assets/` (pelo menos um arquivo é necessário por causa do `//go:embed assets/*`) ou para `~/.local/share/fuoda.sh/sounds/`.
2. Compile e instale:

```bash
make install
```

## Uso

```bash
fuoda.sh wow
fuoda.sh --sync --volume 80 warning
fuoda.sh list
```
