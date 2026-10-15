package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestRun_WritesTheNoticesOfAzt(t *testing.T) {
	notices := filepath.Join(t.TempDir(), "THIRD_PARTY_NOTICES")
	if err := run(t.Context(), notices, []string{"github.com/ingvarch/azt/cmd/azt"}); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(notices)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(data), "Third-party notices for azt") {
		t.Errorf("notices lack the header: %.200s", data)
	}

	// The Azure SDK is MIT, which asks to keep its notice.
	re := regexp.MustCompile(`(?m)^github\.com/Azure/azure-sdk-for-go/sdk/azcore v\S+\nLicenses: MIT\nSource: https://proxy\.golang\.org/`)
	if !re.Match(data) {
		t.Errorf("notices lack the azcore entry")
	}
}
