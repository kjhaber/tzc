package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// zonesTestRoot, if non-empty, overrides the config directory (for tests).
var zonesTestRoot string

func zonesFilePath() (string, error) {
	root := zonesTestRoot
	if root == "" {
		var err error
		root, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	return filepath.Join(root, "tzc", "zones"), nil
}

// loadExtraZoneNames reads persisted IANA zone ids (one per line). Missing file is not an error.
func loadExtraZoneNames() ([]string, error) {
	path, err := zonesFilePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []string
	seen := map[string]struct{}{}
	sc := bufio.NewScanner(strings.NewReader(string(data)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if _, ok := seen[line]; ok {
			continue
		}
		seen[line] = struct{}{}
		out = append(out, line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func saveExtraZoneNames(names []string) error {
	path, err := zonesFilePath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	var b strings.Builder
	for _, n := range names {
		b.WriteString(strings.TrimSpace(n))
		b.WriteByte('\n')
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(b.String()), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
