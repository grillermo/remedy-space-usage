package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Memory is the last selection: location to ticked. Unticks are stored too, so
// a default-ticked row you unticked stays unticked, while a location never seen
// before still gets its default.
type Memory map[string]bool

func memoryPath(home string) string {
	return filepath.Join(home, ".config", "remedy-space-usage", "selection.json")
}

// loadMemory returns an empty Memory for a missing file. A corrupt file also
// gives an empty Memory, plus an error for the caller to report.
func loadMemory(path string) (Memory, error) {
	m := Memory{}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return Memory{}, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}

// save replaces the file in one step, so a crash can't leave half a selection.
func (m Memory) save(path string) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (m Memory) apply(items []*Item) {
	for _, it := range items {
		if ticked, ok := m[it.location()]; ok {
			it.Ticked = ticked
		}
	}
}
