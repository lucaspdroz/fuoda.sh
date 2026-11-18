package sshserver

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/ssh"
	"github.com/charmbracelet/wish"
	"github.com/charmbracelet/wish/bubbletea"
	"github.com/creack/pty"
)

type SoundCallback func(event string)

type Credential struct {
	Length int    `json:"length"`
	Hash   string `json:"hash"`
}

type ServerConfig struct {
	Addr        string
	HostKeyPath string
	CredPath    string
	OnEvent     SoundCallback
}

func DefaultCredPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".handshake.json"
	}
	return filepath.Join(home, ".handshake.json")
}

func saveCredential(path string, cred Credential) error {
	data, err := json.MarshalIndent(cred, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func loadCredential(path string) (Credential, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Credential{}, err
	}
	var cred Credential
	err = json.Unmarshal(data, &cred)
	return cred, err
}

func hashSequence(keys []string) string {
	joined := strings.Join(keys, ":")
	sum := sha256.Sum256([]byte(joined))
	return hex.EncodeToString(sum[:])
}

func verifySequence(keys []string, expectedHash string) bool {
	actualHash := hashSequence(keys)
	return subtle.ConstantTimeCompare([]byte(actualHash), []byte(expectedHash)) == 1
}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FF87"))
	boxStyle   = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 3).BorderForeground(lipgloss.Color("#7D56F4"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF4C4C")).Bold(true)
	warnStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD700")).Bold(true)
)

type keygenModel struct {
	keys []string
	done bool
}

func (m keygenModel) Init() tea.Cmd { return nil }

func (m keygenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		key := strings.ToLower(msg.String())
		switch key {
		case "ctrl+c", "esc":
			m.keys = nil
			return m, tea.Quit
		case "enter":
			if len(m.keys) > 0 {
				m.done = true
				return m, tea.Quit
			}
		case "backspace":
			if len(m.keys) > 0 {
				m.keys = m.keys[:len(m.keys)-1]
			}
		default:
			m.keys = append(m.keys, key)
		}
	}
	return m, nil
}

func (m keygenModel) View() string {
	seqDisplay := "(nenhuma tecla ainda)"
	if len(m.keys) > 0 {
		seqDisplay = strings.Join(m.keys, " → ")
	}
	content := fmt.Sprintf(
		"%s\n\nPressione a sequência secreta desejada:\n\n  %s\n\n[Backspace] apaga | [Enter] salva | [Esc] cancela",
		warnStyle.Render("🔑 KEYGEN DE CREDENCIAL SSH"),
		seqDisplay,
	)
	return "\n" + boxStyle.Render(content) + "\n"
}

func RunKeygen(credPath string, onEvent SoundCallback) error {
	p := tea.NewProgram(keygenModel{})
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	res := finalModel.(keygenModel)
	if !res.done || len(res.keys) == 0 {
		if onEvent != nil {
			onEvent("warning")
		}
		fmt.Println("Operação cancelada.")
		return nil
	}

	cred := Credential{
		Length: len(res.keys),
		Hash:   hashSequence(res.keys),
	}

	if err := saveCredential(credPath, cred); err != nil {
		if onEvent != nil {
			onEvent("error")
		}
		return fmt.Errorf("erro ao salvar credencial em %s: %w", credPath, err)
	}

	if onEvent != nil {
		onEvent("success")
	}
	fmt.Printf("\n✔ Credencial criptografada salva com sucesso em: %s (permissão 0600)\n", credPath)
	fmt.Printf("  Tamanho: %d teclas | Hash: %s...\n\n", cred.Length, cred.Hash[:16])
	return nil
}

type tickMsg time.Time

type handshakeModel struct {
	progress     []string
	expectedLen  int
	expectedHash string
	passed       bool
	failed       bool
	timedOut     bool
	timeLeft     int
}

func initialHandshakeModel(cred Credential) handshakeModel {
	return handshakeModel{
		progress:     []string{},
		expectedLen:  cred.Length,
		expectedHash: cred.Hash,
		timeLeft:     10,
	}
}

func (m handshakeModel) Init() tea.Cmd { return tick() }

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m handshakeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		m.timeLeft--
		if m.timeLeft <= 0 {
			m.failed = true
			m.timedOut = true
			return m, tea.Quit
		}
		return m, tick()

	case tea.KeyMsg:
		key := strings.ToLower(msg.String())
		if key == "ctrl+c" || key == "esc" {
			m.failed = true
			return m, tea.Quit
		}

		m.progress = append(m.progress, key)
		if len(m.progress) == m.expectedLen {
			if verifySequence(m.progress, m.expectedHash) {
				m.passed = true
			} else {
				m.failed = true
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m handshakeModel) View() string {
	var dots []string
	for i := 0; i < m.expectedLen; i++ {
		if i < len(m.progress) {
			dots = append(dots, "●")
		} else {
			dots = append(dots, "○")
		}
	}

	content := fmt.Sprintf(
		"%s\n\nInsira a sequência de handshake:\n\n  %s\n\nTempo restante: %ds",
		titleStyle.Render("🔐 SSH HANDSHAKE PROTOCOL"),
		strings.Join(dots, "  "),
		m.timeLeft,
	)

	if m.failed {
		content += "\n\n" + errStyle.Render("✖ HANDSHAKE REJEITADO")
	}

	return "\n" + boxStyle.Render(content) + "\n"
}

func defaultShell() string {
	if runtime.GOOS == "darwin" {
		return "/bin/zsh"
	}
	return "/bin/bash"
}

func handshakeMiddleware(cred Credential, onEvent SoundCallback) wish.Middleware {
	return func(next ssh.Handler) ssh.Handler {
		return func(s ssh.Session) {
			ptyReq, winCh, isPty := s.Pty()
			if !isPty {
				if onEvent != nil {
					onEvent("warning")
				}
				wish.Println(s, "Este servidor exige um terminal interativo (PTY).")
				s.Exit(1)
				return
			}

			if onEvent != nil {
				onEvent("notification")
			}

			p := tea.NewProgram(initialHandshakeModel(cred), bubbletea.MakeOptions(s)...)
			finalModel, err := p.Run()
			if err != nil {
				if onEvent != nil {
					onEvent("error")
				}
				s.Exit(1)
				return
			}

			res := finalModel.(handshakeModel)
			if !res.passed {
				if onEvent != nil {
					if res.timedOut {
						onEvent("faaah")
					} else {
						onEvent("error")
					}
				}
				wish.Println(s, "\r\nAcesso negado. Encerrando conexão...")
				s.Exit(1)
				return
			}

			if onEvent != nil {
				onEvent("success")
			}

			wish.Println(s, "\r\n✔ Handshake validado! Injetando ambiente e abrindo shell...\r\n")

			cmd := exec.Command(defaultShell())
			cmd.Env = append(os.Environ(),
				fmt.Sprintf("TERM=%s", ptyReq.Term),
				"HANDSHAKE_VERIFIED=true",
				fmt.Sprintf("HANDSHAKE_USER=%s", s.User()),
			)

			ptmx, err := pty.Start(cmd)
			if err != nil {
				if onEvent != nil {
					onEvent("error")
				}
				wish.Println(s, "Erro ao iniciar shell:", err)
				s.Exit(1)
				return
			}
			defer ptmx.Close()

			go func() {
				for win := range winCh {
					_ = pty.Setsize(ptmx, &pty.Winsize{
						Rows: uint16(win.Height),
						Cols: uint16(win.Width),
					})
				}
			}()

			go func() { _, _ = io.Copy(ptmx, s) }()
			_, _ = io.Copy(s, ptmx)

			_ = cmd.Wait()
			next(s)
		}
	}
}

func RunServer(cfg ServerConfig) error {
	cred, err := loadCredential(cfg.CredPath)
	if err != nil {
		if cfg.OnEvent != nil {
			cfg.OnEvent("error")
		}
		return fmt.Errorf("nenhuma credencial encontrada em %s; rode primeiro: fuoda.sh ssh --keygen", cfg.CredPath)
	}

	srv, err := wish.NewServer(
		wish.WithAddress(cfg.Addr),
		wish.WithHostKeyPath(cfg.HostKeyPath),
		wish.WithMiddleware(
			handshakeMiddleware(cred, cfg.OnEvent),
		),
	)
	if err != nil {
		if cfg.OnEvent != nil {
			cfg.OnEvent("error")
		}
		return fmt.Errorf("erro ao criar servidor: %w", err)
	}

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	log.Printf("Servidor SSH Handshake rodando em %s (credencial carregada de %s)...", cfg.Addr, cfg.CredPath)
	if cfg.OnEvent != nil {
		cfg.OnEvent("notification")
	}

	errCh := make(chan error, 1)
	go func() {
		if err = srv.ListenAndServe(); err != nil && !errors.Is(err, ssh.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		if cfg.OnEvent != nil {
			cfg.OnEvent("error")
		}
		return err
	case <-done:
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(ctx)
	}
}
