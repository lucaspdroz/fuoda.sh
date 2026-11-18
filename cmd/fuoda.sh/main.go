package main

import (
	"embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"fuoda.sh/internal/audio"
	"fuoda.sh/internal/config"
	"fuoda.sh/internal/sshserver"
)

//go:embed assets/*
var embeddedAssets embed.FS

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "play":
		err = playCommand(os.Args[2:])
	case "trigger":
		err = triggerCommand(os.Args[2:])
	case "list":
		err = listCommand()
	case "themes", "theme":
		err = themesCommand(os.Args[2:])
	case "keygen", "--keygen":
		err = sshCommand(append([]string{"--keygen"}, os.Args[2:]...))
	case "ssh":
		err = sshCommand(os.Args[2:])
	case "__play":
		err = internalPlay(os.Args[2:])
	case "help", "--help", "-h":
		usage()
	default:
		err = playCommand(os.Args[1:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fuoda.sh:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Print(`fuoda.sh - efeitos sonoros para scripts, terminal e servidor SSH

Uso:
  fuoda.sh [--volume 0..100] [--sync] <evento|arquivo.mp3>
  fuoda.sh play [--volume 0..100] [--sync] <evento|arquivo.mp3>
  fuoda.sh trigger [--volume 0..100] [--sync] <evento>
  fuoda.sh list

Servidor SSH com Handshake Sonoro:
  fuoda.sh ssh [--keygen] [--cred caminho.json] [--addr :2222] [--volume 100]

Áudios externos:
  ~/.local/share/fuoda.sh/sounds/

Mapeamentos:
  ~/.config/fuoda.sh/events.conf
`)
}

func playEventAsync(event string, volume int) {
	name, err := config.EventFile(event)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[evento de audio %q não mapeado: ] %v\n", event, err)
		return
	}
	path, err := resolveAudio(name)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[falha ao localizar audio:] %q: %v\n", name, err)
		return
	}

	go func() {
		defer func() {
			if filepath.Dir(path) == os.TempDir() && strings.HasPrefix(filepath.Base(path), "fuoda.sh-") {
				_ = os.Remove(path)
			}
		}()
		if err := audio.PlayFile(path, volume); err != nil {
			fmt.Fprintf(os.Stderr, "[audio] erro ao reproduzir %q: %v\n", path, err)
		}
	}()
}

func sshCommand(args []string) error {
	fs := flag.NewFlagSet("ssh", flag.ContinueOnError)
	keygenFlag := fs.Bool("keygen", false, "Gera e salva uma nova credencial criptografada")
	credFile := fs.String("cred", sshserver.DefaultCredPath(), "Caminho do arquivo de credencial")
	addr := fs.String("addr", ":2222", "Endereço/porta de escuta do servidor SSH")
	home, _ := os.UserHomeDir()
	defaultHostKey := filepath.Join(home, ".config", "fuoda.sh", "host_ed25519")
	hostKey := fs.String("host-key", defaultHostKey, "Caminho da chave de host SSH")
	volume := fs.Int("volume", 100, "Volume dos efeitos sonoros (0 a 100)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	onEvent := func(event string) {
		playEventAsync(event, *volume)
	}

	if *keygenFlag {
		return sshserver.RunKeygen(*credFile, onEvent)
	}

	return sshserver.RunServer(sshserver.ServerConfig{
		Addr:        *addr,
		HostKeyPath: *hostKey,
		CredPath:    *credFile,
		OnEvent:     onEvent,
	})
}

func playCommand(args []string) error {
	fs := flag.NewFlagSet("play", flag.ContinueOnError)
	volume := fs.Int("volume", 100, "volume de 0 a 100")
	syncPlay := fs.Bool("sync", false, "aguardar o fim da reprodução")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("informe o caminho de um arquivo MP3 ou nome do evento")
	}
	target := fs.Arg(0)
	if mapped, err := config.EventFile(target); err == nil {
		target = mapped
	}
	path, err := resolveAudio(target)
	if err != nil {
		return err
	}
	return launch(path, *volume, *syncPlay)
}

func themesCommand(args []string) error {
	available := listAllThemes()
	active := config.ActiveTheme()

	if len(args) == 0 {
		fmt.Printf("Tema ativo: %s\n\nTemas disponíveis:\n", active)
		for _, t := range available {
			marker := "  "
			if t == active {
				marker = "* "
			}
			fmt.Printf("%s%s\n", marker, t)
		}
		fmt.Println("\nPara trocar: fuoda.sh themes <nome-do-tema>")
		return nil
	}

	chosen := strings.TrimSpace(args[0])
	found := false
	for _, t := range available {
		if t == chosen {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("tema %q não encontrado; rode 'fuoda.sh themes' para ver os disponíveis", chosen)
	}

	if err := config.SetActiveTheme(chosen); err != nil {
		return err
	}
	fmt.Printf("✔ Tema alterado para: %s\n", chosen)
	return nil
}

func listAllThemes() []string {
	seen := map[string]struct{}{"default": {}}

	if entries, err := embeddedAssets.ReadDir("assets/themes"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				seen[e.Name()] = struct{}{}
			}
		}
	}
	if entries, err := os.ReadDir(config.ThemesDir()); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				seen[e.Name()] = struct{}{}
			}
		}
	}

	themes := make([]string, 0, len(seen))
	for name := range seen {
		themes = append(themes, name)
	}
	sort.Strings(themes)
	return themes
}

func triggerCommand(args []string) error {
	fs := flag.NewFlagSet("trigger", flag.ContinueOnError)
	volume := fs.Int("volume", 100, "volume de 0 a 100")
	syncPlay := fs.Bool("sync", false, "aguardar o fim da reprodução")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("informe o nome do evento")
	}
	event := fs.Arg(0)
	name, err := config.EventFile(event)
	if err != nil {
		return err
	}
	path, err := resolveAudio(name)
	if err != nil {
		return fmt.Errorf("evento %q: %w", event, err)
	}
	return launch(path, *volume, *syncPlay)
}

func listCommand() error {
	for _, line := range config.ListEvents() {
		fmt.Println(line)
	}
	return nil
}

func resolveAudio(name string) (string, error) {
	if filepath.IsAbs(name) {
		if isFile(name) {
			return name, nil
		}
		return "", fmt.Errorf("arquivo não encontrado: %s", name)
	}
	if isFile(name) {
		abs, _ := filepath.Abs(name)
		return abs, nil
	}

	baseName := filepath.Base(name)
	if filepath.Ext(baseName) == "" {
		baseName += ".mp3"
	}

	theme := config.ActiveTheme()
	if theme != "default" {
		// 1. Tema externo (~/.local/share/fuoda.sh/themes/<tema>/<arquivo>)
		extThemeFile := filepath.Join(config.ThemesDir(), theme, baseName)
		if isFile(extThemeFile) {
			return extThemeFile, nil
		}
		// 2. Tema embutido (assets/themes/<tema>/<arquivo>)
		if tmpPath, ok := extractEmbedded("assets/themes/" + theme + "/" + baseName); ok {
			return tmpPath, nil
		}
	}

	// 3. Áudios externos gerais (~/.local/share/fuoda.sh/sounds/<arquivo>)
	external := filepath.Join(config.SoundDir(), baseName)
	if isFile(external) {
		return external, nil
	}

	// 4. Assets padrão embutidos (assets/<arquivo>)
	if tmpPath, ok := extractEmbedded("assets/" + baseName); ok {
		return tmpPath, nil
	}

	return "", fmt.Errorf("áudio %q não encontrado (tema: %s); procure em %s", name, theme, config.SoundDir())
}

func extractEmbedded(embeddedPath string) (string, bool) {
	data, err := embeddedAssets.ReadFile(embeddedPath)
	if err != nil {
		return "", false
	}
	tmp, err := os.CreateTemp("", "fuoda.sh-*.mp3")
	if err != nil {
		return "", false
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return "", false
	}
	if err := tmp.Close(); err != nil {
		return "", false
	}
	return tmp.Name(), true
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func launch(path string, volume int, syncPlay bool) error {
	if volume < 0 || volume > 100 {
		return errors.New("volume deve estar entre 0 e 100")
	}
	if syncPlay {
		return audio.PlayFile(path, volume)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, "__play", path, strconv.Itoa(volume))
	cmd.Stdin = nil
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

func internalPlay(args []string) error {
	if len(args) != 2 {
		return errors.New("uso interno inválido")
	}
	volume, err := strconv.Atoi(args[1])
	if err != nil {
		return err
	}
	defer func() {
		if filepath.Dir(args[0]) == os.TempDir() && strings.HasPrefix(filepath.Base(args[0]), "fuoda.sh-") {
			_ = os.Remove(args[0])
		}
	}()
	return audio.PlayFile(args[0], volume)
}
