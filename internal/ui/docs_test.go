package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/ui/views"
)

const usageDoc = "../../docs/usage.md"

func readDoc(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// keysRow returns the line of the doc's keys table whose first cell starts with key.
func keysRow(doc, key string) string {
	for _, l := range strings.Split(doc, "\n") {
		if strings.HasPrefix(l, "| `"+key+"` |") {
			return l
		}
	}
	return ""
}

func TestUsageCommandRowListsEveryTypeCommand(t *testing.T) {
	row := keysRow(readDoc(t, usageDoc), ":")
	if row == "" {
		t.Fatal("docs/usage.md has no row for the : key")
	}
	for _, c := range views.TypeCommands() {
		if !strings.Contains(row, "`"+c.Name+"`") {
			t.Errorf("docs/usage.md row lacks the command %q: %s", c.Name, row)
		}
	}
}

func TestDocsWriteCtrlDash(t *testing.T) {
	docs, err := filepath.Glob("../../docs/*.md")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range append(docs, "../../README.md") {
		if strings.Contains(readDoc(t, path), "ctrl+") {
			t.Errorf("%s writes ctrl+ instead of ctrl-", filepath.Base(path))
		}
	}
}

func TestUsageNamesTheSubscriptionKeys(t *testing.T) {
	doc := readDoc(t, usageDoc)
	if row := keysRow(doc, "0-9"); !strings.Contains(row, "`:subs`") {
		t.Errorf("docs/usage.md has no keys row for 0-9 that names :subs: %q", row)
	}
	for _, want := range []string{"Each tab can show one subscription", "number of rows"} {
		if !strings.Contains(doc, want) {
			t.Errorf("docs/usage.md lacks %q", want)
		}
	}
}
