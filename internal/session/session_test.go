package session

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/ui/views"
)

func sample() Session {
	return Session{
		Version: Version,
		Tabs: []Tab{{
			Name: "work", Zoomed: true, Focus: 1,
			Layout: &Node{
				Split: "h", Ratio: 0.3,
				A: &Node{Stack: []views.Spec{{Kind: "subscriptions"}}},
				B: &Node{Stack: []views.Spec{
					{Kind: "subscriptions"},
					{Kind: "resourceGroups", Subscription: "id-1", SubscriptionName: "Dev", Filter: "rg"},
				}},
			},
		}},
		Active:         0,
		SidebarVisible: true,
	}
}

func TestDefaultPath(t *testing.T) {
	t.Run("xdg", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "/state")
		got, err := DefaultPath()
		if err != nil || got != filepath.Join("/state", "azt", "session.json") {
			t.Errorf("got %q, %v", got, err)
		}
	})
	t.Run("home", func(t *testing.T) {
		t.Setenv("XDG_STATE_HOME", "")
		// os.UserHomeDir reads USERPROFILE on Windows.
		home := "HOME"
		if runtime.GOOS == "windows" {
			home = "USERPROFILE"
		}
		t.Setenv(home, "/home/u")
		got, err := DefaultPath()
		if err != nil || got != filepath.Join("/home/u", ".local", "state", "azt", "session.json") {
			t.Errorf("got %q, %v", got, err)
		}
	})
}

func TestRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "azt", "session.json")
	want := sample()
	if err := Save(path, want); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestSaveOverwrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	first := sample()
	first.Active = 5
	for _, s := range []Session{first, sample()} {
		if err := Save(path, s); err != nil {
			t.Fatal(err)
		}
	}
	got, err := Load(path)
	if err != nil || got.Active != 0 {
		t.Errorf("got Active %d, %v", got.Active, err)
	}
}

func TestSaveModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no POSIX file modes")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "azt")
	path := filepath.Join(dir, "session.json")
	if err := Save(path, sample()); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]fs.FileMode{dir: 0o700, path: 0o600} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != want {
			t.Errorf("%s: mode %o, want %o", p, got, want)
		}
	}
}

func TestSaveLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	if err := Save(filepath.Join(dir, "session.json"), sample()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "session.json" {
		t.Errorf("directory holds %v", entries)
	}
}

func TestSaveFailureLeavesNoTempFile(t *testing.T) {
	dir := t.TempDir()
	// A directory at the target makes the rename fail.
	path := filepath.Join(dir, "session.json")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := Save(path, sample()); err == nil {
		t.Fatal("Save over a directory succeeded")
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("directory holds %v", entries)
	}
}

func TestLoadMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "none.json"))
	if !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v, want fs.ErrNotExist", err)
	}
}

func TestLoadBroken(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil || errors.Is(err, fs.ErrNotExist) {
		t.Errorf("err = %v", err)
	}
}

func TestLoadOtherVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.json")
	s := sample()
	s.Version = Version + 1
	b, _ := json.Marshal(s)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Errorf("err = %v", err)
	}
}

func TestJSONNames(t *testing.T) {
	b, err := json.Marshal(sample())
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{`"version"`, `"tabs"`, `"active"`, `"sidebarVisible"`, `"zoomed"`, `"focus"`, `"layout"`, `"split"`, `"ratio"`, `"stack"`} {
		if !strings.Contains(string(b), name) {
			t.Errorf("%s missing in %s", name, b)
		}
	}
}
