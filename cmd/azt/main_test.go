package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/version"
)

func TestVersionFlag(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"--version"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	if got, want := out.String(), "azt "+version.Full()+"\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestHelpFlag(t *testing.T) {
	var errOut bytes.Buffer
	err := run([]string{"-h"}, &bytes.Buffer{}, &errOut)
	if !errors.Is(err, flag.ErrHelp) {
		t.Errorf("err = %v, want flag.ErrHelp", err)
	}
	if !strings.Contains(errOut.String(), "-version") {
		t.Errorf("usage does not list -version:\n%s", errOut.String())
	}
}

func TestUnknownFlag(t *testing.T) {
	if err := run([]string{"--nope"}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Error("err = nil, want an error for an unknown flag")
	}
}
