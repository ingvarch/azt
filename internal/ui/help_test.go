package ui

import (
	"strings"
	"testing"
)

func TestHelpListsResourceTabKeys(t *testing.T) {
	for _, line := range helpLines() {
		if strings.Contains(line, "tab / shift+tab") && strings.Contains(line, "next / previous tab of a resource") {
			return
		}
	}
	t.Errorf("help lacks the tab keys:\n%s", strings.Join(helpLines(), "\n"))
}
