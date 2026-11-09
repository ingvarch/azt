# Development

How to build, run and check azt, and where its code lives. How to propose a
change is in [CONTRIBUTING.md](../CONTRIBUTING.md); how a release is made is in
[releasing.md](releasing.md).

## Prerequisites

- Go, at least the version named in `go.mod`.
- The Azure CLI, signed in with `az login`. azt reads with that account.
- For `make lint`: golangci-lint, the version named in the Makefile. CI runs
  the same one.
- For `make dist`: goreleaser, the version named in `.tool-versions`, and
  [syft](https://github.com/anchore/syft) for the SBOMs.

## Run

```sh
az login
make run
```

`make run` builds `bin/azt` and starts it.

## Checks

```sh
make check   # fmt, vet, lint, licenses, test, test on the oldest Go, build
make dist    # the whole release, published nowhere
```

- `make test-min-go` runs the tests on the go line of `go.mod`, the oldest Go
  azt supports. Go downloads that toolchain the first time.
- `make licenses` fails when a module azt links has a license outside the list
  in `internal/licenses`; `make notices` also writes `THIRD_PARTY_NOTICES`,
  which the release ships.
- `make dist` skips signing: that needs the release workflow.

Every push runs the same checks in CI: lint and licenses on Linux, the tests
on Linux, macOS and Windows with the newest Go and on Linux with the oldest
one, govulncheck, and the same release snapshot as `make dist`.

## Project structure

| Path | What it holds |
| --- | --- |
| `cmd/azt` | the `main` package |
| `internal/azure` | Resource Graph queries and Resource Manager GET requests |
| `internal/ui` | the Bubble Tea interface: tabs, panes, dialogs, help, mouse |
| `internal/ui/views` | the views a pane can show, one file per resource kind |
| `internal/ui/data` | fetches Azure data for the views and caches it |
| `internal/ui/sidebar` | the explorer tree |
| `internal/layout` | the immutable split tree of panes |
| `internal/session` | saves the layout on exit and reads it on start |
| `internal/version` | what the build stamps into the binary |
| `internal/licenses` | checks the licenses of linked modules and writes the notices; not linked into azt |
| `internal/buildconfig` | tests that the Makefile, `.goreleaser.yaml` and the workflows agree |
