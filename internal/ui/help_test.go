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
		if strings.Contains(line, "tab / shift-tab") && strings.Contains(line, "next / previous tab of a resource") {
			return
		}
	}
	t.Errorf("help lacks the tab keys:\n%s", strings.Join(helpLines(), "\n"))
}

func TestHelpListsPeriodKey(t *testing.T) {
	for _, line := range helpLines() {
		if strings.HasPrefix(strings.TrimSpace(line), "p ") && strings.Contains(line, "period of the Monitoring tab") {
			return
		}
	}
	t.Errorf("help lacks the period key:\n%s", strings.Join(helpLines(), "\n"))
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

func TestHelpBoxFitsAStandardTerminal(t *testing.T) {
	if w := helpMaxWidth + helpMargin; w > 80 {
		t.Errorf("the help box needs %d columns, want at most 80", w)
	}
}

func TestHelpListsTheSubscriptionKeys(t *testing.T) {
	for _, line := range helpLines() {
		if strings.HasPrefix(strings.TrimSpace(line), "0-9 ") && strings.Contains(line, "tab's subscription: 0 all, 1-9 as numbered in :subs") {
			return
		}
	}
	t.Errorf("help lacks the subscription keys:\n%s", strings.Join(helpLines(), "\n"))
}

func TestHelpListsTheCopyKeysForPanesAndExplorer(t *testing.T) {
	n := 0
	for _, line := range helpLines() {
		if strings.HasPrefix(strings.TrimSpace(line), "y / Y ") && strings.Contains(line, "copy the ID / name of the selected item") {
			n++
		}
	}
	if n != 2 {
		t.Errorf("help lists the copy keys %d times, want 2 (panes, explorer):\n%s", n, strings.Join(helpLines(), "\n"))
	}
}
