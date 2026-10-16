package ui

import (
	"os"
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/ui/views"
)

func TestReadmeCommandRowListsEveryTypeCommand(t *testing.T) {
	raw, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	var row string
	for _, l := range strings.Split(string(raw), "\n") {
		if strings.HasPrefix(l, "| `:` |") {
			row = l
		}
	}
	if row == "" {
		t.Fatal("README has no row for the : key")
	}
	for _, c := range views.TypeCommands() {
		if !strings.Contains(row, "`"+c.Name+"`") {
			t.Errorf("README row lacks the command %q: %s", c.Name, row)
		}
	}
}

func TestReadmeWritesCtrlDash(t *testing.T) {
	raw, err := os.ReadFile("../../README.md")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "ctrl+") {
		t.Error("README writes ctrl+ instead of ctrl-")
	}
}
