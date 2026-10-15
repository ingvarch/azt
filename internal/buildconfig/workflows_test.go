package buildconfig_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// repoFile reads a file by its path from the repository root.
func repoFile(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(name)))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return data
}

// workflow is the part of a GitHub Actions workflow the tests look at.
type workflow struct {
	text []byte // the file as written, for the comments the parser drops

	Permissions map[string]string `json:"permissions"`
	Jobs        map[string]job    `json:"jobs"`
}

type job struct {
	file, name string // where the job is, for failure messages

	RunsOn   string `json:"runs-on"`
	Strategy struct {
		Matrix map[string]any `json:"matrix"`
	} `json:"strategy"`
	Steps []step `json:"steps"`
}

type step struct {
	If   string            `json:"if"`
	Uses string            `json:"uses"`
	Run  string            `json:"run"`
	With map[string]any    `json:"with"`
	Env  map[string]string `json:"env"`
}

// with returns an input of the step as text, or "" when the step does not set it.
func (s step) with(key string) string {
	v, ok := s.With[key]
	if !ok {
		return ""
	}
	return fmt.Sprint(v)
}

// usesAction reports whether the step uses the action at any version.
func (s step) usesAction(action string) bool {
	return strings.HasPrefix(s.Uses, action+"@")
}

// where names the job in failure messages.
func (j job) where() string {
	return fmt.Sprintf("%s: job %s", j.file, j.name)
}

// uses reports whether a step of the job uses the action.
func (j job) uses(action string) bool {
	return slices.ContainsFunc(j.Steps, func(s step) bool { return s.usesAction(action) })
}

// runsGo reports whether a run step of the job calls the go command.
func (j job) runsGo() bool {
	for _, s := range j.Steps {
		for line := range strings.Lines(s.Run) {
			if strings.HasPrefix(strings.TrimSpace(line), "go ") {
				return true
			}
		}
	}
	return false
}

// workflowNames lists the workflow files.
func workflowNames(t *testing.T) []string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join("..", "..", ".github", "workflows"))
	if err != nil {
		t.Fatalf("reading the workflows: %v", err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	if len(names) == 0 {
		t.Fatal("there are no workflows")
	}
	return names
}

// loadWorkflow parses one workflow file.
func loadWorkflow(t *testing.T, name string) workflow {
	t.Helper()
	return parseWorkflow(t, name, repoFile(t, ".github/workflows/"+name))
}

// parseWorkflow parses the text of the workflow file name.
func parseWorkflow(t *testing.T, name string, text []byte) workflow {
	t.Helper()
	w := workflow{text: text}
	if err := yaml.Unmarshal(text, &w); err != nil {
		t.Fatalf("parsing %s: %v", name, err)
	}
	for jobName, j := range w.Jobs {
		j.file, j.name = name, jobName
		w.Jobs[jobName] = j
	}
	return w
}

// workflowJob returns a job of a workflow file, which must exist.
func workflowJob(t *testing.T, file, name string) job {
	t.Helper()
	j, ok := loadWorkflow(t, file).Jobs[name]
	if !ok {
		t.Fatalf("%s has no job %q", file, name)
	}
	return j
}

// stepUsing returns the job's step that uses the action, which must exist.
func stepUsing(t *testing.T, j job, action string) step {
	t.Helper()
	for _, s := range j.Steps {
		if s.usesAction(action) {
			return s
		}
	}
	t.Fatalf("%s: no step uses %s", j.where(), action)
	return step{}
}

// ciLintVersion is the golangci-lint version CI lints with, without the v.
func ciLintVersion(t *testing.T) string {
	t.Helper()
	lint := workflowJob(t, "ci.yml", "lint")
	return strings.TrimPrefix(stepUsing(t, lint, "golangci/golangci-lint-action").with("version"), "v")
}

// matrixValues returns a list from a job's matrix.
func matrixValues(t *testing.T, j job, key string) []string {
	t.Helper()
	raw, ok := j.Strategy.Matrix[key].([]any)
	if !ok {
		t.Fatalf("%s: the matrix has no list %q", j.where(), key)
	}
	values := make([]string, 0, len(raw))
	for _, v := range raw {
		values = append(values, fmt.Sprint(v))
	}
	return values
}

// matrixIncludes returns the maps under include in a job's matrix, or none when the key is absent.
func matrixIncludes(t *testing.T, j job) []map[string]any {
	t.Helper()
	raw, ok := j.Strategy.Matrix["include"]
	if !ok {
		return nil
	}
	list, ok := raw.([]any)
	if !ok {
		t.Fatalf("%s: the matrix include is not a list", j.where())
	}
	entries := make([]map[string]any, 0, len(list))
	for _, e := range list {
		entry, ok := e.(map[string]any)
		if !ok {
			t.Fatalf("%s: the matrix include holds %v, not a map", j.where(), e)
		}
		entries = append(entries, entry)
	}
	return entries
}

// testCombinations returns the os/go pairs a job runs: os x go, plus the include entries that extend none of those
// combinations. GitHub matches an include entry only against os x go.
func testCombinations(t *testing.T, j job) []string {
	t.Helper()
	var pairs []string
	for _, osName := range matrixValues(t, j, "os") {
		for _, goRelease := range matrixValues(t, j, "go") {
			pairs = append(pairs, osName+"/"+goRelease)
		}
	}
	n := len(pairs)
	for _, e := range matrixIncludes(t, j) {
		extends := false
		for _, p := range pairs[:n] {
			osName, goRelease, _ := strings.Cut(p, "/")
			extends = extends || (fmt.Sprint(e["os"]) == osName && fmt.Sprint(e["go"]) == goRelease)
		}
		if !extends {
			pairs = append(pairs, fmt.Sprintf("%v/%v", e["os"], e["go"]))
		}
	}
	return pairs
}

// stringsIn returns every string in a parsed YAML value, at any depth.
func stringsIn(v any) []string {
	var found []string
	switch v := v.(type) {
	case string:
		found = append(found, v)
	case []any:
		for _, e := range v {
			found = append(found, stringsIn(e)...)
		}
	case map[string]any:
		for _, e := range v {
			found = append(found, stringsIn(e)...)
		}
	}
	return found
}

// commitSHA matches a full commit id.
var commitSHA = regexp.MustCompile(`^[0-9a-f]{40}$`)

// actionVersions lists the action of every step as action@version. A commit ref is named by the "# v..." comment on
// its line, or by nothing.
func (w workflow) actionVersions() []string {
	var versions []string
	for _, j := range w.Jobs {
		for _, s := range j.Steps {
			if s.Uses == "" {
				continue
			}
			action, ref, _ := strings.Cut(s.Uses, "@")
			if commitSHA.MatchString(ref) {
				ref = w.versionComment(s.Uses)
			}
			versions = append(versions, action+"@"+ref)
		}
	}
	return versions
}

// versionComment returns the "# v..." comment that follows uses on its line, or "".
func (w workflow) versionComment(uses string) string {
	m := regexp.MustCompile(regexp.QuoteMeta(uses) + `[ \t]+#[ \t]*(v\S+)`).FindSubmatch(w.text)
	if m == nil {
		return ""
	}
	return string(m[1])
}

func TestActionVersionsReadEveryStep(t *testing.T) {
	// The pin check sees only what this returns, so no step may drop out.
	const sha = "3d3c42e5aac5ba805825da76410c181273ba90b1"
	cases := []struct {
		name, steps string
		want        []string
	}{
		{"tag", "- uses: actions/checkout@v7", []string{"actions/checkout@v7"}},
		{"commit with a version", "- uses: actions/checkout@" + sha + " # v7.0.1", []string{"actions/checkout@v7.0.1"}},
		{"quoted", `- uses: "actions/checkout@v4"`, []string{"actions/checkout@v4"}},
		{
			"commit without a version", "- uses: actions/checkout@" + sha + "\n      # v4.2.0\n      - uses: actions/setup-go@v7",
			[]string{"actions/checkout@", "actions/setup-go@v7"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			text := "jobs:\n  build:\n    steps:\n      " + c.steps + "\n"
			if got := parseWorkflow(t, c.name, []byte(text)).actionVersions(); !slices.Equal(got, c.want) {
				t.Errorf("actionVersions of\n%s= %q, want %q", text, got, c.want)
			}
		})
	}
}

func TestWorkflowsPinTheRunner(t *testing.T) {
	// A floating label changes the machine under the build on a date somebody else picks.
	for _, name := range workflowNames(t) {
		for _, j := range loadWorkflow(t, name).Jobs {
			runners := []string{j.RunsOn}
			if strings.Contains(j.RunsOn, "${{") {
				runners = append(runners, stringsIn(j.Strategy.Matrix)...)
			}
			for _, r := range runners {
				if strings.Contains(r, "-latest") {
					t.Errorf("%s runs on the floating label %s", j.where(), r)
				}
			}
		}
	}
}

func TestCITestsTheNewestGoEverywhereAndTheModuleGoOnLinux(t *testing.T) {
	test := workflowJob(t, "ci.yml", "test")
	want := []string{"ubuntu-26.04/stable", "macos-15/stable", "windows-2025/stable", "ubuntu-26.04/go.mod"}
	got := testCombinations(t, test)
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("%s: combinations %q, want %q", test.where(), got, want)
	}
	// setup-go gets one input: it warns when it gets both.
	setup := stepUsing(t, test, "actions/setup-go")
	if got, want := setup.with("go-version-file"), "${{ matrix.go == 'go.mod' && 'go.mod' || '' }}"; got != want {
		t.Errorf("%s: go-version-file %q, want %q", test.where(), got, want)
	}
	if got, want := setup.with("go-version"), "${{ matrix.go != 'go.mod' && matrix.go || '' }}"; got != want {
		t.Errorf("%s: go-version %q, want %q", test.where(), got, want)
	}
	var runs []string
	for _, s := range test.Steps {
		if s.Run != "" {
			runs = append(runs, s.Run)
		}
	}
	if !slices.Equal(runs, []string{"go test -race ./..."}) {
		t.Errorf("%s runs %q, want go test -race ./...", test.where(), runs)
	}
}

func TestCITriesTheReleaseLikeMakeDist(t *testing.T) {
	// A broken release config shows up in the pull request, and make dist builds the same thing.
	j := workflowJob(t, "ci.yml", "snapshot")
	syft := slices.IndexFunc(j.Steps, func(s step) bool { return s.usesAction("anchore/sbom-action/download-syft") })
	gr := slices.IndexFunc(j.Steps, func(s step) bool { return s.usesAction("goreleaser/goreleaser-action") })
	if syft < 0 || gr < 0 || syft > gr {
		t.Fatalf("%s must install syft before it runs goreleaser (syft step %d, goreleaser step %d)", j.where(), syft, gr)
	}
	goreleaser := j.Steps[gr]
	args := goreleaser.with("args")
	if dist := recipe(t, "dist"); len(dist) != 1 || dist[0] != "goreleaser "+args {
		t.Errorf("%s runs goreleaser %q, make dist runs %q", j.where(), args, dist)
	}
	for _, flag := range []string{"--snapshot", "--skip=sign"} {
		if !slices.Contains(strings.Fields(args), flag) {
			t.Errorf("%s runs goreleaser %q without %s", j.where(), args, flag)
		}
	}
	if got := goreleaser.with("version-file"); got != ".tool-versions" {
		t.Errorf("%s: goreleaser version-file %q, want .tool-versions", j.where(), got)
	}
}

func TestCIRunsGovulncheck(t *testing.T) {
	if !workflowJob(t, "ci.yml", "govulncheck").uses("golang/govulncheck-action") {
		t.Error("ci.yml: the govulncheck job does not use golang/govulncheck-action")
	}
}

func TestGoJobsSetUpGo(t *testing.T) {
	// Without setup-go a job builds with whatever Go the runner image carries. govulncheck-action sets up Go itself.
	for _, name := range workflowNames(t) {
		for _, j := range loadWorkflow(t, name).Jobs {
			needsGo := j.uses("golangci/golangci-lint-action") || j.uses("goreleaser/goreleaser-action") || j.runsGo()
			if needsGo && !j.uses("actions/setup-go") {
				t.Errorf("%s runs Go without actions/setup-go", j.where())
			}
		}
	}
}

func TestReleaseWorkflowRunsOnVersionTags(t *testing.T) {
	if !strings.Contains(string(repoFile(t, ".github/workflows/release.yml")), `tags: ["v*"]`) {
		t.Error("release.yml does not run on v* tags")
	}
	// Keyless signing needs the job's OIDC token.
	if p := loadWorkflow(t, "release.yml").Permissions; p["contents"] != "write" || p["id-token"] != "write" {
		t.Errorf("permissions = %v, want contents and id-token write", p)
	}
}

func TestReleaseWorkflowSetsUpWhatTheReleaseNeeds(t *testing.T) {
	j := workflowJob(t, "release.yml", "release")
	// Keyless signing publishes the workflow identity, and brew cannot download private release assets.
	if len(j.Steps) == 0 || j.Steps[0].If != "github.event.repository.private" || !strings.Contains(j.Steps[0].Run, "exit 1") {
		t.Errorf("%s does not refuse a private repository in its first step", j.where())
	}
	if got := stepUsing(t, j, "actions/setup-go").with("go-version"); got != "stable" {
		t.Errorf("%s sets up Go %q, want stable", j.where(), got)
	}
	stepUsing(t, j, "sigstore/cosign-installer")
	stepUsing(t, j, "anchore/sbom-action/download-syft")
	gr := stepUsing(t, j, "goreleaser/goreleaser-action")
	if got := gr.with("version-file"); got != ".tool-versions" {
		t.Errorf("%s: goreleaser version-file %q, want .tool-versions", j.where(), got)
	}
	for _, secret := range []string{
		"GITHUB_TOKEN", "HOMEBREW_TAP_GITHUB_TOKEN", "MACOS_SIGN_P12", "MACOS_SIGN_PASSWORD",
		"MACOS_NOTARY_ISSUER_ID", "MACOS_NOTARY_KEY_ID", "MACOS_NOTARY_KEY",
	} {
		if gr.Env[secret] != "${{ secrets."+secret+" }}" {
			t.Errorf("%s does not pass %s to goreleaser", j.where(), secret)
		}
	}
}

func TestReleaseWorkflowPinsActionsByCommit(t *testing.T) {
	// The release job holds write tokens and every release secret, so a moved tag must not change its code.
	pinned := regexp.MustCompile(`^[\w.-]+/[\w.-]+(/[\w./-]+)?@[0-9a-f]{40}$`)
	w := loadWorkflow(t, "release.yml")
	for _, j := range w.Jobs {
		for _, s := range j.Steps {
			if s.Uses != "" && !pinned.MatchString(s.Uses) {
				t.Errorf("%s uses %s, not a full commit", j.where(), s.Uses)
			}
		}
	}
	// The comment names the release the commit belongs to.
	tagged := regexp.MustCompile(`@v\d+\.\d+\.\d+$`)
	for _, used := range w.actionVersions() {
		if !tagged.MatchString(used) {
			t.Errorf("release.yml uses %s without a # vX.Y.Z comment", used)
		}
	}
}
