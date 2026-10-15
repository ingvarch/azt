package buildconfig_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/licenses"
)

// listEntry returns an entry of a .goreleaser.yaml list that is either text or a map, such as a hook (cmd) or an
// archive file (src).
func listEntry(v any, key string) string {
	switch v := v.(type) {
	case string:
		return v
	case map[string]any:
		s, _ := v[key].(string)
		return s
	}
	return ""
}

func TestMakeNoticesChecksWhatMakeLicensesChecks(t *testing.T) {
	// Both run the same check on the same packages; notices also writes the file the release ships.
	notices, check := oneCommand(t, "notices"), oneCommand(t, "licenses")
	if !strings.Contains(notices, " -notices "+noticesFile+" ") {
		t.Fatalf("make notices runs %q, which does not write %s", notices, noticesFile)
	}
	if got := strings.Replace(notices, " -notices "+noticesFile, "", 1); got != check {
		t.Errorf("make notices runs %q, make licenses runs %q: they check different things", notices, check)
	}
}

func TestReleaseWritesTheNoticesBeforeItBuilds(t *testing.T) {
	// A before hook runs in the snapshot too, so every pull request writes the notices and checks the licenses.
	want := oneCommand(t, "notices")
	hooks := release(t).Before.Hooks
	if !slices.ContainsFunc(hooks, func(h any) bool { return listEntry(h, "cmd") == want }) {
		t.Errorf(".goreleaser.yaml before hooks %v do not run %q, as make notices does", hooks, want)
	}
}

func TestReleaseArchivesShipTheLicenseAndTheNotices(t *testing.T) {
	archives := release(t).Archives
	if len(archives) == 0 {
		t.Fatal(".goreleaser.yaml has no archives")
	}
	for i, a := range archives {
		for _, file := range []string{"LICENSE", "README.md", noticesFile} {
			if !slices.ContainsFunc(a.Files, func(f any) bool { return listEntry(f, "src") == file }) {
				t.Errorf("archive %d does not ship %s: files %v", i, file, a.Files)
			}
		}
	}
}

func TestReleasePublishesTheNotices(t *testing.T) {
	cfg := release(t)
	notices := func(f extraFile) bool { return f.Glob == noticesFile }
	if !slices.ContainsFunc(cfg.Release.ExtraFiles, notices) {
		t.Errorf("release extra_files %+v do not publish %s", cfg.Release.ExtraFiles, noticesFile)
	}
	if !slices.ContainsFunc(cfg.Checksum.ExtraFiles, notices) {
		t.Errorf("checksum extra_files %+v do not list %s", cfg.Checksum.ExtraFiles, noticesFile)
	}
}

func TestCIChecksTheLicensesLikeMakeLicenses(t *testing.T) {
	want := oneCommand(t, "licenses")
	lint := workflowJob(t, "ci.yml", "lint")
	i := slices.IndexFunc(lint.Steps, func(s step) bool { return s.Run == want })
	if i < 0 {
		t.Fatalf("%s does not run %q, as make licenses does", lint.where(), want)
	}
	// A failed linter before it must not hide a license failure; the job still fails.
	if got := lint.Steps[i].If; got != "success() || failure()" {
		t.Errorf("%s checks the licenses with if %q, want \"success() || failure()\"", lint.where(), got)
	}
}

func TestLicenseCheckCoversEveryReleasePlatform(t *testing.T) {
	// A module linked on only one platform still ships in that archive.
	var want []string
	for _, target := range azt(t).Targets {
		if p := platform(t, target); !slices.Contains(want, p) {
			want = append(want, p)
		}
	}
	got := slices.Clone(licenses.Platforms)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(want, got) {
		t.Errorf("licenses.Platforms = %q, the release targets are %q", got, want)
	}
}

func TestGitIgnoresWhatTheReleaseGenerates(t *testing.T) {
	// A tracked copy would go stale, and an untracked one would make the tree dirty.
	ignored := map[string]bool{}
	for line := range strings.Lines(string(repoFile(t, ".gitignore"))) {
		ignored[strings.TrimSpace(line)] = true
	}
	for _, path := range []string{"/" + noticesFile, "/dist/"} {
		if !ignored[path] {
			t.Errorf(".gitignore does not ignore %s", path)
		}
	}
}
