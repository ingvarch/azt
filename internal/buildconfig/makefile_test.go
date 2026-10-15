package buildconfig_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// noticesFile is the third-party notices file make notices writes and the release ships.
const noticesFile = "THIRD_PARTY_NOTICES"

// recipe returns the commands of a Makefile target, one per line, without the leading tab.
func recipe(t *testing.T, target string) []string {
	t.Helper()
	lines := strings.Split(string(repoFile(t, "Makefile")), "\n")
	for i, line := range lines {
		if line != target+":" && !strings.HasPrefix(line, target+": ") {
			continue
		}
		var commands []string
		for _, c := range lines[i+1:] {
			if !strings.HasPrefix(c, "\t") {
				break
			}
			commands = append(commands, strings.TrimPrefix(c, "\t"))
		}
		return commands
	}
	t.Fatalf("the Makefile has no target %q", target)
	return nil
}

// oneCommand returns the only command of a Makefile target, with its variables expanded.
func oneCommand(t *testing.T, target string) string {
	t.Helper()
	commands := recipe(t, target)
	if len(commands) != 1 {
		t.Fatalf("make %s runs %q, want one command", target, commands)
	}
	return expand(t, commands[0])
}

// makeVariable returns the value of a variable the Makefile sets with = or :=, its continued lines joined by spaces.
func makeVariable(t *testing.T, name string) string {
	t.Helper()
	lines := strings.Split(string(repoFile(t, "Makefile")), "\n")
	assign := regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `\s*[:?]?=\s*(.*)$`)
	for i, line := range lines {
		m := assign.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		value := m[1]
		for rest := lines[i+1:]; strings.HasSuffix(value, `\`) && len(rest) > 0; rest = rest[1:] {
			value = strings.TrimSuffix(value, `\`) + " " + rest[0]
		}
		return strings.Join(strings.Fields(value), " ")
	}
	t.Fatalf("the Makefile does not set %s", name)
	return ""
}

// expand replaces the $(NAME) variables of a Makefile line with their values.
func expand(t *testing.T, s string) string {
	t.Helper()
	return regexp.MustCompile(`\$\((\w+)\)`).ReplaceAllStringFunc(s, func(ref string) string {
		return makeVariable(t, ref[2:len(ref)-1])
	})
}

// makefileLintVersion is the golangci-lint version the Makefile insists on.
func makefileLintVersion(t *testing.T) string {
	t.Helper()
	m := regexp.MustCompile(`(?m)^GOLANGCI_LINT_VERSION := (\S+)$`).FindSubmatch(repoFile(t, "Makefile"))
	if m == nil {
		t.Fatal("the Makefile sets no GOLANGCI_LINT_VERSION")
	}
	return string(m[1])
}

// stamped returns the variables that -X flags set, such as github.com/ingvarch/azt/internal/version.Version.
func stamped(ldflags string) []string {
	var vars []string
	for _, m := range regexp.MustCompile(`-X\s+([^=\s]+)=`).FindAllStringSubmatch(ldflags, -1) {
		vars = append(vars, m[1])
	}
	slices.Sort(vars)
	return vars
}

func TestMakefileLintsWithTheVersionCIUses(t *testing.T) {
	// Another version finds other things than CI does.
	if got, want := makefileLintVersion(t), ciLintVersion(t); got != want {
		t.Errorf("the Makefile wants golangci-lint %s, CI lints with %s", got, want)
	}
}

func TestMakefileCheckRunsEveryGate(t *testing.T) {
	m := regexp.MustCompile(`(?m)^check: (.+)$`).FindSubmatch(repoFile(t, "Makefile"))
	if m == nil {
		t.Fatal("the Makefile has no check target")
	}
	want := []string{"fmt", "vet", "lint", "licenses", "test", "test-min-go", "build"}
	if got := strings.Fields(string(m[1])); !slices.Equal(got, want) {
		t.Errorf("make check runs %q, want %q", got, want)
	}
	for _, gate := range want {
		if len(recipe(t, gate)) == 0 {
			t.Errorf("make %s has no commands", gate)
		}
	}
}

func TestMakefileStampsWhatTheReleaseStamps(t *testing.T) {
	// A development build reports its version, commit and date as a release does.
	got := stamped(expand(t, makeVariable(t, "LDFLAGS")))
	want := stamped(strings.Join(azt(t).Ldflags, " "))
	if len(want) != 3 {
		t.Fatalf("the release stamps %q, want three variables", want)
	}
	if !slices.Equal(got, want) {
		t.Errorf("the Makefile stamps %q, the release stamps %q", got, want)
	}
}

func TestMakeCleanRemovesWhatTheBuildWrites(t *testing.T) {
	removed := strings.Fields(oneCommand(t, "clean"))
	for _, generated := range []string{"bin", "dist", noticesFile} {
		if !slices.Contains(removed, generated) {
			t.Errorf("make clean runs %q, which leaves %s", strings.Join(removed, " "), generated)
		}
	}
}
