// Package shell implementa "fuoda.sh shell": um wrapper de PTY que roda o
// shell do usuário (zsh, bash, fish...) normalmente e toca sons quando a
// saída do terminal contém marcadores como "[play success]".
//
//	terminal ⇄ fuoda.sh shell (PTY) ⇄ zsh
//	                 │
//	          vê toda a saída → marcador → som
//
// A saída passa sem nenhuma alteração: o wrapper só observa.
package shell

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// EnvMarker é definido dentro do shell filho. Serve para detectar um
// fuoda.sh shell dentro de outro (o que tocaria cada som duas vezes).
const EnvMarker = "FUODA_SHELL"

// Config configura o wrapper.
type Config struct {
	// Shell a executar. Vazio: $SHELL, ou /bin/sh.
	Shell string
	// Args extras para o shell (por exemplo "-l").
	Args []string
	// OnEvent é chamado para cada marcador encontrado.
	OnEvent func(event string)
	// MinInterval é o intervalo mínimo entre dois sons do mesmo evento, para
	// que um "cat" de um arquivo cheio de marcadores não vire uma rajada.
	MinInterval time.Duration
}

// Run executa o shell dentro de um PTY e devolve o código de saída dele.
func Run(cfg Config) (int, error) {
	sh := cfg.Shell
	if sh == "" {
		sh = os.Getenv("SHELL")
	}
	if sh == "" {
		sh = "/bin/sh"
	}
	if os.Getenv(EnvMarker) != "" {
		return 1, errors.New("já estamos dentro de um fuoda.sh shell")
	}

	// Sem terminal (script, pipe): não há o que embrulhar; vira o shell.
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		argv := append([]string{sh}, cfg.Args...)
		return 1, syscall.Exec(sh, argv, os.Environ())
	}

	cmd := exec.Command(sh, cfg.Args...)
	cmd.Env = append(os.Environ(), EnvMarker+"=1")
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return 1, fmt.Errorf("iniciando %s: %w", sh, err)
	}
	defer ptmx.Close()

	// Tamanho da janela: agora e a cada SIGWINCH.
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for range winch {
			_ = pty.InheritSize(os.Stdin, ptmx)
		}
	}()
	winch <- syscall.SIGWINCH

	old, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		_ = cmd.Process.Kill()
		return 1, fmt.Errorf("modo raw: %w", err)
	}
	defer func() { _ = term.Restore(int(os.Stdin.Fd()), old) }()

	det := &Detector{OnEvent: throttle(cfg.OnEvent, cfg.MinInterval)}

	// Entrada: teclado → shell. Essa goroutine fica bloqueada no Read do
	// stdin depois que o shell sai; o processo termina logo em seguida.
	go func() { _, _ = io.Copy(ptmx, os.Stdin) }()

	// Saída: shell → terminal, com uma cópia para o detector. O io.Copy
	// termina quando o shell fecha o PTY (EIO no Linux).
	_, _ = io.Copy(io.MultiWriter(os.Stdout, det), ptmx)

	if err := cmd.Wait(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

// throttle descarta repetições do mesmo evento dentro de min.
func throttle(f func(string), min time.Duration) func(string) {
	if f == nil {
		return nil
	}
	var mu sync.Mutex
	last := map[string]time.Time{}
	return func(ev string) {
		mu.Lock()
		now := time.Now()
		if t, ok := last[ev]; ok && now.Sub(t) < min {
			mu.Unlock()
			return
		}
		last[ev] = now
		mu.Unlock()
		f(ev)
	}
}
