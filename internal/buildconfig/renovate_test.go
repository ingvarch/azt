package buildconfig_test

import (
	"encoding/json"
	"io/fs"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
)

// renovateFile is where the Renovate app reads the repository's config.
const renovateFile = ".github/renovate.json"

// customManager is the part of a Renovate custom manager the tests look at.
type customManager struct {
	CustomType          string   `json:"customType"`
	ManagerFilePatterns []string `json:"managerFilePatterns"`
	MatchStrings        []string `json:"matchStrings"`
	DepNameTemplate     string   `json:"depNameTemplate"`
}

// renovateConfig is the part of the Renovate config the tests look at.
type renovateConfig struct {
	PackageRules   []map[string]any `json:"packageRules"`
	CustomManagers []customManager  `json:"customManagers"`
}

// renovate parses the Renovate config.
func renovate(t *testing.T) renovateConfig {
	t.Helper()
	var cfg renovateConfig
	if err := json.Unmarshal(repoFile(t, renovateFile), &cfg); err != nil {
		t.Fatalf("parsing %s: %v", renovateFile, err)
	}
	return cfg
}

// toolVersion returns the version .tool-versions pins for a tool, read the way asdf and goreleaser-action read it.
func toolVersion(t *testing.T, tool string) string {
	t.Helper()
	for line := range strings.Lines(string(repoFile(t, ".tool-versions"))) {
		if fields := strings.Fields(line); len(fields) >= 2 && fields[0] == tool {
			return fields[1]
		}
	}
	t.Fatalf(".tool-versions pins no %s", tool)
	return ""
}

// managerFiles lists the repository files, as paths from the root, that a managerFilePatterns entry selects. Only the
// /regex/ form is read.
func managerFiles(t *testing.T, pattern string) []string {
	t.Helper()
	if len(pattern) < 2 || !strings.HasPrefix(pattern, "/") || !strings.HasSuffix(pattern, "/") {
		t.Fatalf("file pattern %q is not a /regex/, the only form the tests read", pattern)
	}
	re, err := regexp.Compile(pattern[1 : len(pattern)-1])
	if err != nil {
		t.Fatalf("file pattern %q: %v", pattern, err)
	}
	root := filepath.Join("..", "..")
	var files []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if !d.IsDir() && re.MatchString(filepath.ToSlash(rel)) {
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("listing the repository: %v", err)
	}
	return files
}

// checkMatchString fails unless the pattern matches the file once and captures the pinned version as currentValue.
func checkMatchString(t *testing.T, tool, file, pattern, pinned string) {
	t.Helper()
	// Renovate reads matchStrings as RE2, the syntax of Go's regexp.
	re, err := regexp.Compile(pattern)
	if err != nil {
		t.Errorf("%s: %q: %v", tool, pattern, err)
		return
	}
	found := re.FindAllSubmatch(repoFile(t, file), -1)
	i := re.SubexpIndex("currentValue")
	switch {
	case len(found) != 1:
		t.Errorf("%s: %q matches %s %d times, want once", tool, pattern, file, len(found))
	case i < 0:
		t.Errorf("%s: %q captures no currentValue", tool, pattern)
	case string(found[0][i]) != pinned:
		t.Errorf("%s: %q reads %q from %s, the build uses %q", tool, pattern, found[0][i], file, pinned)
	}
}

// pinnedVersions reads, for each tool a custom manager updates (by its depNameTemplate), the version the build uses.
var pinnedVersions = map[string]func(*testing.T) string{
	"golangci-lint": makefileLintVersion,
	"goreleaser":    func(t *testing.T) string { return toolVersion(t, "goreleaser") },
}

func TestRenovateCustomManagersFindThePins(t *testing.T) {
	// Renovate skips a pin its pattern no longer matches without a word, so an edit that breaks the match fails here.
	updated := map[string]bool{}
	for _, m := range renovate(t).CustomManagers {
		pinned, ok := pinnedVersions[m.DepNameTemplate]
		switch {
		case m.CustomType != "regex":
			t.Errorf("custom manager %q is %s; the tests read only regex managers", m.DepNameTemplate, m.CustomType)
			continue
		case !ok:
			t.Errorf("custom manager %q: no test reads the version it updates", m.DepNameTemplate)
			continue
		}
		updated[m.DepNameTemplate] = true
		want := pinned(t)
		for _, pattern := range m.ManagerFilePatterns {
			files := managerFiles(t, pattern)
			if len(files) == 0 {
				t.Errorf("%s: file pattern %s matches no file", m.DepNameTemplate, pattern)
			}
			for _, file := range files {
				for _, s := range m.MatchStrings {
					checkMatchString(t, m.DepNameTemplate, file, s, want)
				}
			}
		}
	}
	for tool := range pinnedVersions {
		if !updated[tool] {
			t.Errorf("no custom manager updates %s", tool)
		}
	}
}

func TestGoModKeepsTheGoPickedByHand(t *testing.T) {
	// The go line is the oldest Go azt supports, and a dependency update must not move it.
	mod, err := modfile.Parse("go.mod", repoFile(t, "go.mod"), nil)
	if err != nil {
		t.Fatalf("parsing go.mod: %v", err)
	}
	if mod.Go == nil || mod.Go.Version != "1.26.0" {
		t.Errorf("go.mod go line = %v, want go 1.26.0", mod.Go)
	}
	if mod.Toolchain != nil {
		t.Errorf("go.mod has a toolchain line: %s", mod.Toolchain.Name)
	}
}

func TestRenovateLeavesTheGoLineAlone(t *testing.T) {
	// Renovate names both the go line and a toolchain line dependency go.
	want := map[string]any{
		"matchManagers": []any{"gomod"},
		"matchDepNames": []any{"go"},
		"matchDepTypes": []any{"golang", "toolchain"},
		"enabled":       false,
	}
	for _, r := range renovate(t).PackageRules {
		delete(r, "description")
		// A rule with one more match field would cover less than it says.
		if reflect.DeepEqual(r, want) {
			return
		}
	}
	t.Errorf("%s has no package rule that is exactly %v", renovateFile, want)
}
