// Package session saves the layout of azt on exit and reads it on start.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ingvarch/azt/internal/ui/views"
)

// Version is the file format version; Load rejects any other.
const Version = 1

// Session is the saved state of the whole UI.
type Session struct {
	Version        int   `json:"version"`
	Tabs           []Tab `json:"tabs"`
	Active         int   `json:"active"`
	SidebarVisible bool  `json:"sidebarVisible"`
	// SidebarWidth is the explorer width in columns; 0 means the default.
	SidebarWidth int `json:"sidebarWidth,omitempty"`
}

// Tab is one tab: its pane layout and view state.
type Tab struct {
	Name   string `json:"name,omitempty"`
	Zoomed bool   `json:"zoomed,omitempty"`
	// Focus is the index of the focused pane in layout pane order.
	Focus  int   `json:"focus,omitempty"`
	Layout *Node `json:"layout"`
}

// Node is a split ("h" or "v") of A and B, or a pane with a stack of views.
type Node struct {
	Split string  `json:"split,omitempty"`
	Ratio float64 `json:"ratio,omitempty"`
	A     *Node   `json:"a,omitempty"`
	B     *Node   `json:"b,omitempty"`
	// Stack is the pane's views, bottom to top.
	Stack []views.Spec `json:"stack,omitempty"`
}

// DefaultPath is $XDG_STATE_HOME/azt/session.json, or under ~/.local/state.
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_STATE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".local", "state")
	}
	return filepath.Join(dir, "azt", "session.json"), nil
}

// Load reads the session file at path.
func Load(path string) (Session, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Session{}, err
	}
	var s Session
	if err := json.Unmarshal(b, &s); err != nil {
		return Session{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if s.Version != Version {
		return Session{}, fmt.Errorf("session version %d, want %d", s.Version, Version)
	}
	return s, nil
}

// Save writes s through a temp file and a rename, so a crash never leaves half a file.
func Save(path string, s Session) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	name, err := writeTemp(dir, b)
	if err != nil {
		return err
	}
	if err := os.Rename(name, path); err != nil {
		return errors.Join(err, os.Remove(name))
	}
	return nil
}

// writeTemp writes b to a new file in dir (mode 0600) and returns its name.
func writeTemp(dir string, b []byte) (string, error) {
	tmp, err := os.CreateTemp(dir, "session-*.tmp")
	if err != nil {
		return "", err
	}
	_, err = tmp.Write(b)
	if err == nil {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", errors.Join(err, os.Remove(tmp.Name()))
	}
	return tmp.Name(), nil
}
