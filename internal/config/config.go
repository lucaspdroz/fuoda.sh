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
	"warning":      "mac-quack.mp3",
	"notification": "notification.mp3",
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "fuoda.sh")
}

func SoundDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "fuoda.sh", "sounds")
}

func ThemesDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "fuoda.sh", "themes")
}

func ActiveTheme() string {
	data, err := os.ReadFile(filepath.Join(ConfigDir(), "theme"))
	if err != nil {
		return "default"
	}
	theme := strings.TrimSpace(string(data))
	if theme == "" {
		return "default"
	}
	return theme
}

func SetActiveTheme(theme string) error {
	if err := os.MkdirAll(ConfigDir(), 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(ConfigDir(), "theme"), []byte(strings.TrimSpace(theme)+"\n"), 0644)
}

func EventFile(event string) (string, error) {
	if value, ok := readEvents(filepath.Join(ConfigDir(), "events.conf"))[event]; ok {
		return value, nil
	}
	theme := ActiveTheme()
	if theme != "default" {
		if value, ok := readEvents(filepath.Join(ThemesDir(), theme, "events.conf"))[event]; ok {
			return value, nil
		}
		if _, ok := defaultEvents[event]; ok {
			return event + ".mp3", nil
		}
	}
	if value, ok := defaultEvents[event]; ok {
		return value, nil
	}
	return "", errors.New("evento desconhecido; configure-o em ~/.config/fuoda.sh/events.conf")
}

func ListEvents() []string {
	merged := make(map[string]string, len(defaultEvents))
	theme := ActiveTheme()

	for k, v := range defaultEvents {
		if theme != "default" {
			merged[k] = k + ".mp3"
		} else {
			merged[k] = v
		}
	}
	if theme != "default" {
		for k, v := range readEvents(filepath.Join(ThemesDir(), theme, "events.conf")) {
			merged[k] = v
		}
	}
	for k, v := range readEvents(filepath.Join(ConfigDir(), "events.conf")) {
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

func readEvents(path string) map[string]string {
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
	if scanner.Err() != nil {
		return nil
	}
	return events
}
