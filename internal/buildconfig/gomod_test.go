package buildconfig_test

import (
	"testing"

	"golang.org/x/mod/modfile"
)

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
