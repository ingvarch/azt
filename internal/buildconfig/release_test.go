package buildconfig_test

import (
	"slices"
	"strings"
	"testing"

	"sigs.k8s.io/yaml"
)

// releaseConfig is the part of .goreleaser.yaml the tests look at.
type releaseConfig struct {
	Before struct {
		Hooks []any `json:"hooks"`
	} `json:"before"`
	Builds   []releaseBuild   `json:"builds"`
	Archives []releaseArchive `json:"archives"`
	Checksum struct {
		NameTemplate string      `json:"name_template"`
		ExtraFiles   []extraFile `json:"extra_files"`
	} `json:"checksum"`
	Signs []struct {
		Cmd       string   `json:"cmd"`
		Signature string   `json:"signature"`
		Args      []string `json:"args"`
		Artifacts string   `json:"artifacts"`
	} `json:"signs"`
	SBOMs []struct {
		Artifacts string `json:"artifacts"`
	} `json:"sboms"`
	Notarize struct {
		MacOS []struct {
			Enabled string `json:"enabled"`
			Sign    struct {
				Certificate string `json:"certificate"`
				Password    string `json:"password"`
			} `json:"sign"`
			Notarize struct {
				IssuerID string `json:"issuer_id"`
				KeyID    string `json:"key_id"`
				Key      string `json:"key"`
			} `json:"notarize"`
		} `json:"macos"`
	} `json:"notarize"`
	NFPMs []struct {
		Formats  []string      `json:"formats"`
		Contents []packageFile `json:"contents"`
	} `json:"nfpms"`
	HomebrewCasks []struct {
		SkipUpload string `json:"skip_upload"`
		Repository struct {
			Owner  string `json:"owner"`
			Name   string `json:"name"`
			Branch string `json:"branch"`
			Token  string `json:"token"`
		} `json:"repository"`
	} `json:"homebrew_casks"`
	Changelog struct {
		Use string `json:"use"`
	} `json:"changelog"`
	Release struct {
		ExtraFiles []extraFile `json:"extra_files"`
	} `json:"release"`
}

// extraFile is a file the release publishes beside what it builds.
type extraFile struct {
	Glob string `json:"glob"`
}

// releaseBuild is a build of .goreleaser.yaml: one binary for several platforms.
type releaseBuild struct {
	Main    string   `json:"main"`
	Binary  string   `json:"binary"`
	Env     []string `json:"env"`
	Flags   []string `json:"flags"`
	Targets []string `json:"targets"`
	Ldflags []string `json:"ldflags"`
}

// releaseArchive is an archive of .goreleaser.yaml and how it is packed.
type releaseArchive struct {
	Files           []any    `json:"files"`
	Formats         []string `json:"formats"`
	FormatOverrides []struct {
		Goos    string   `json:"goos"`
		Formats []string `json:"formats"`
	} `json:"format_overrides"`
}

// packageFile is a file the deb and rpm packages install.
type packageFile struct {
	Src string `json:"src"`
	Dst string `json:"dst"`
}

// release parses .goreleaser.yaml.
func release(t *testing.T) releaseConfig {
	t.Helper()
	var cfg releaseConfig
	if err := yaml.Unmarshal(repoFile(t, ".goreleaser.yaml"), &cfg); err != nil {
		t.Fatalf("parsing .goreleaser.yaml: %v", err)
	}
	return cfg
}

// azt returns the only build of .goreleaser.yaml.
func azt(t *testing.T) releaseBuild {
	t.Helper()
	builds := release(t).Builds
	if len(builds) != 1 {
		t.Fatalf(".goreleaser.yaml has %d builds, want one", len(builds))
	}
	return builds[0]
}

// platform returns the goos/goarch of a GoReleaser target, such as linux/arm64 for linux_arm64_v8.0.
func platform(t *testing.T, target string) string {
	t.Helper()
	parts := strings.SplitN(target, "_", 3)
	if len(parts) < 2 {
		t.Fatalf("target %q is not goos_goarch", target)
	}
	return parts[0] + "/" + parts[1]
}

func TestReleaseBuildsAztStatic(t *testing.T) {
	b := azt(t)
	if b.Main != "./cmd/azt" || b.Binary != "azt" {
		t.Errorf("the build is %q as %q, want ./cmd/azt as azt", b.Main, b.Binary)
	}
	// A static binary runs on any system, whatever libc it has.
	if !slices.Contains(b.Env, "CGO_ENABLED=0") {
		t.Errorf("build env %q lacks CGO_ENABLED=0", b.Env)
	}
	if !slices.Contains(b.Flags, "-trimpath") {
		t.Errorf("build flags %q lack -trimpath", b.Flags)
	}
}

func TestReleaseStampsTheVersion(t *testing.T) {
	// The release says which tag, commit and date it was built from; the tag keeps its v.
	const pkg = "-X github.com/ingvarch/azt/internal/version."
	ldflags := strings.Join(azt(t).Ldflags, " ")
	for _, want := range []string{pkg + "Version=v{{ .Version }}", pkg + "Commit=", pkg + "Date="} {
		if !strings.Contains(ldflags, want) {
			t.Errorf("ldflags %q lack %q", ldflags, want)
		}
	}
}

func TestReleaseZipsForWindows(t *testing.T) {
	archives := release(t).Archives
	if len(archives) != 1 {
		t.Fatalf(".goreleaser.yaml has %d archives, want one", len(archives))
	}
	a := archives[0]
	if !slices.Equal(a.Formats, []string{"tar.gz"}) || len(a.FormatOverrides) != 1 ||
		a.FormatOverrides[0].Goos != "windows" || !slices.Equal(a.FormatOverrides[0].Formats, []string{"zip"}) {
		t.Errorf("archive = %+v, want tar.gz with a zip for windows", a)
	}
}

func TestReleaseNamesTheChecksums(t *testing.T) {
	if got := release(t).Checksum.NameTemplate; got != "checksums.txt" {
		t.Errorf("checksum name_template = %q, want checksums.txt", got)
	}
}

func TestReleaseSignsTheChecksumsWithCosign(t *testing.T) {
	// Keyless: the bundle carries the certificate and the signature.
	signs := release(t).Signs
	if len(signs) != 1 || signs[0].Cmd != "cosign" || signs[0].Artifacts != "checksum" ||
		signs[0].Signature != "${artifact}.sigstore.json" {
		t.Fatalf("signs = %+v, want cosign over the checksums into ${artifact}.sigstore.json", signs)
	}
	want := []string{"sign-blob", "--bundle=${signature}", "${artifact}", "--yes"}
	if !slices.Equal(signs[0].Args, want) {
		t.Errorf("cosign args = %q, want %q", signs[0].Args, want)
	}
}

func TestReleaseWritesAnSBOMPerArchive(t *testing.T) {
	sboms := release(t).SBOMs
	if len(sboms) != 1 || sboms[0].Artifacts != "archive" {
		t.Errorf("sboms = %+v, want one over the archives", sboms)
	}
}

func TestReleaseNotarizesForMacOS(t *testing.T) {
	macos := release(t).Notarize.MacOS
	if len(macos) != 1 {
		t.Fatalf("notarize.macos = %+v, want one", macos)
	}
	m := macos[0]
	// A snapshot has no Apple keys; a release without them fails instead of shipping a binary macOS refuses.
	if m.Enabled != "{{ not .IsSnapshot }}" {
		t.Errorf("notarize.macos enabled = %q, want only outside snapshots", m.Enabled)
	}
	got := []string{
		m.Sign.Certificate, m.Sign.Password, m.Notarize.IssuerID, m.Notarize.KeyID, m.Notarize.Key,
	}
	want := []string{
		"{{ .Env.MACOS_SIGN_P12 }}", "{{ .Env.MACOS_SIGN_PASSWORD }}", "{{ .Env.MACOS_NOTARY_ISSUER_ID }}",
		"{{ .Env.MACOS_NOTARY_KEY_ID }}", "{{ .Env.MACOS_NOTARY_KEY }}",
	}
	if !slices.Equal(got, want) {
		t.Errorf("notarize.macos reads %q, want %q", got, want)
	}
}

func TestReleasePacksDebAndRpmWithTheLicense(t *testing.T) {
	nfpms := release(t).NFPMs
	if len(nfpms) != 1 || !slices.Equal(nfpms[0].Formats, []string{"deb", "rpm"}) {
		t.Fatalf("nfpms = %+v, want deb and rpm", nfpms)
	}
	// A package is a copy, and the licenses of azt and of what it links ask for their texts in every copy.
	for _, file := range []string{"LICENSE", noticesFile} {
		if !slices.Contains(nfpms[0].Contents, packageFile{Src: file, Dst: "/usr/share/doc/azt/" + file}) {
			t.Errorf("the packages do not ship %s as /usr/share/doc/azt/%s: %+v", file, file, nfpms[0].Contents)
		}
	}
}

func TestReleasePublishesTheCaskToTheTap(t *testing.T) {
	casks := release(t).HomebrewCasks
	if len(casks) != 1 {
		t.Fatalf("homebrew_casks = %+v, want one", casks)
	}
	repo := casks[0].Repository
	if repo.Owner != "ingvarch" || repo.Name != "homebrew-tap" || repo.Branch != "main" ||
		repo.Token != "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}" {
		t.Errorf("cask repository = %+v, want ingvarch/homebrew-tap@main with the tap token", repo)
	}
	// A release candidate stays out of brew upgrade.
	if casks[0].SkipUpload != "auto" {
		t.Errorf("skip_upload = %q, want auto", casks[0].SkipUpload)
	}
}

func TestReleaseNotesComeFromGitHub(t *testing.T) {
	if got := release(t).Changelog.Use; got != "github-native" {
		t.Errorf("changelog use = %q, want github-native", got)
	}
}
