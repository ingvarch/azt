# AGENTS.md

azt is a terminal UI for Azure, written in Go with Bubble Tea. `internal/azure`
reads Azure, `internal/ui` and its `views`, `data` and `sidebar` packages draw
it, and `internal/layout` and `internal/session` hold the tabs and panes. Every
change starts with a failing test, and `make check` is green before every
commit.

- [CONTRIBUTING.md](CONTRIBUTING.md): read before any change. It has the
  test-first rule, the oldest Go line, English-only UI, comments, code from
  other projects, and the commit and pull request format.
- [docs/development.md](docs/development.md): read before building, running or
  checking azt, or looking for where code lives. It has the prerequisites, the
  `make` targets and the project structure.
- [docs/usage.md](docs/usage.md): read before changing a key, a command or a
  view. It has the views, the keys table, the mouse, the session file and what
  azt reads.
- [docs/install.md](docs/install.md): read before changing how azt is
  packaged or installed. It has Homebrew, deb and rpm, archives, `go install`.
- [docs/releasing.md](docs/releasing.md): read before a release or a change to
  `.goreleaser.yaml` or `.github/workflows/release.yml`. It has what a release
  publishes, the steps, the secrets and the failures seen so far.
- [SECURITY.md](SECURITY.md#what-azt-can-reach): read before adding an Azure
  call or a file azt writes. It says what azt reads and writes.

## Commands

`make lint` refuses to run when the installed golangci-lint is not the version
CI uses. To lint with that version without installing it:

```sh
go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0 run ./...
```

## Rules

- azt only reads Azure. A new Azure call is a Resource Graph query or a
  Resource Manager GET request; update
  [SECURITY.md](SECURITY.md#what-azt-can-reach) when the set of calls changes.
- Write Control keys as `ctrl-w` in every text a user reads: the help screen,
  the status bar, the docs. Bubble Tea names them `ctrl+w`, so key names in
  code and tests keep the plus.
- The help screen (`internal/ui/help.go`) and the keys table in
  [docs/usage.md](docs/usage.md#keys) list the same keys; change them
  together.
- UI changes follow k9s and [taws](https://github.com/huseyinbabal/taws):
  fewer, clearer elements, scope on the number keys.
- Issues and pull requests go on the GitHub milestone of the next version.
- The `default-branch` ruleset requires the CI jobs by name: `lint`,
  `test <os> go <go>`, `govulncheck`, `release snapshot`. Renaming a job or
  changing a runner or the Go matrix changes a name; update the ruleset with
  the change, or every pull request waits for a check that never runs.
