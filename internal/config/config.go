package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var defaultEvents = map[string]string{
	"success":      "success.mp3",
	"error":        "error.mp3",
	"fah":          "fah.mp3",
	"warning":      "mac-quack.mp3",
	"wow":          "anime-wow-sound-effect.mp3",
	"bolso":        "bolsonaro-pegaram-meu-telefone.mp3",
	"notification": "notification.mp3",
}

func SoundDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "fuoda.sh", "sounds")
}

func EventFile(event string) (string, error) {
	if value, ok := readEvents()[event]; ok {
		return value, nil
	}
	if value, ok := defaultEvents[event]; ok {
		return value, nil
	}
	return "", errors.New("evento desconhecido; configure-o em ~/.config/fuoda.sh/events.conf")
}

func ListEvents() []string {
	merged := make(map[string]string, len(defaultEvents))
	for k, v := range defaultEvents {
		merged[k] = v
	}
	for k, v := range readEvents() {
		merged[k] = v
	}

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var lines []string
	for _, k := range keys {
		lines = append(lines, fmt.Sprintf("%-14s -> %s", k, merged[k]))
	}
	return lines
}

func readEvents() map[string]string {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".config", "fuoda.sh", "events.conf")
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	events := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			events[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return events
}

func ListSounds() ([]string, error) {
	entries, err := os.ReadDir(SoundDir())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(entry.Name()), ".mp3") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}
