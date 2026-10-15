package ui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/ui/views"
)

func TestHelpListsResourceTabKeys(t *testing.T) {
	for _, line := range helpLines() {
		if strings.Contains(line, "tab / shift+tab") && strings.Contains(line, "next / previous tab of a resource") {
			return
		}
	}
	t.Errorf("help lacks the tab keys:\n%s", strings.Join(helpLines(), "\n"))
}

func TestHelpListsEveryTypeCommand(t *testing.T) {
	var commands []string
	for _, s := range helpSections {
		if s.title == "Commands" {
			for _, e := range s.entries {
				commands = append(commands, strings.Fields(e.keys)...)
			}
		}
	}
	for _, c := range views.TypeCommands() {
		if !slices.Contains(commands, c.Name) {
			t.Errorf("help lacks the command %q", c.Name)
		}
	}
}

func TestHelpLinesFitTheBox(t *testing.T) {
	for _, line := range helpLines() {
		if w := lipgloss.Width(line); w > helpMaxWidth-2 {
			t.Errorf("%d cells, want at most %d: %q", w, helpMaxWidth-2, line)
		}
	}
}
