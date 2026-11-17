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
	"strconv"
	"strings"
	"syscall"

	"fuoda.sh/internal/audio"
	"fuoda.sh/internal/config"
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
	fmt.Print(`fuoda.sh - efeitos sonoros para scripts e aplicações

Uso:
  fuoda.sh [--volume 0..100] [--sync] <evento|arquivo.mp3>
  fuoda.sh play [--volume 0..100] [--sync] <evento|arquivo.mp3>
  fuoda.sh trigger [--volume 0..100] [--sync] <evento>
  fuoda.sh list

Áudios externos:
  ~/.local/share/fuoda.sh/sounds/

Mapeamentos:
  ~/.config/fuoda.sh/events.conf
`)
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
	external := filepath.Join(config.SoundDir(), filepath.Base(name))
	if isFile(external) {
		return external, nil
	}
	// Assets embedded at build time are copied to a temporary file for decoding.
	baseName := filepath.Base(name)

	if filepath.Ext(baseName) == "" {
		baseName += ".mp3"
	}

	embeddedName := "assets/" + baseName
	data, err := embeddedAssets.ReadFile(embeddedName)
	if err == nil {
		tmp, err := os.CreateTemp("", "fuoda.sh-*.mp3")
		if err != nil {
			return "", err
		}
		if _, err := tmp.Write(data); err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
			return "", err
		}
		if err := tmp.Close(); err != nil {
			return "", err
		}
		return tmp.Name(), nil
	}
	return "", fmt.Errorf("áudio %q não encontrado; procure em %s", name, config.SoundDir())
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
	// The detached child owns the playback process; the caller returns immediately.
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
		// Embedded assets are staged in a temporary file for the decoder.
		if filepath.Dir(args[0]) == os.TempDir() && strings.HasPrefix(filepath.Base(args[0]), "fuoda.sh-") {
			_ = os.Remove(args[0])
		}
	}()
	return audio.PlayFile(args[0], volume)
}
