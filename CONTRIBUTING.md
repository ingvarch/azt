# Contributing to azt

Bug reports, ideas and pull requests are welcome. Everyone who takes part
follows the [Code of Conduct](CODE_OF_CONDUCT.md). A security problem is
reported privately, not in an issue: see [SECURITY.md](SECURITY.md).

## Before you write code

For anything bigger than a small fix, open an issue first, so the shape of the
change is agreed before the work is done.

## Build and run

You need Go, at least the version named in `go.mod`, and the Azure CLI signed
in with `az login`. azt reads with that account:

```sh
az login
make run
```

```sh
make check   # fmt, vet, lint, licenses, test, test on the oldest Go, build
make dist    # the whole release, published nowhere
```

- `make lint` needs golangci-lint, the version named in the Makefile; CI runs
  the same one.
- `make test-min-go` runs the tests on the go line of `go.mod`, the oldest Go
  azt supports. Go downloads that toolchain the first time.
- `make licenses` fails when a module azt links has a license outside the list
  in `internal/licenses`; `make notices` also writes `THIRD_PARTY_NOTICES`,
  which the release ships.
- `make dist` needs goreleaser, the version named in `.tool-versions`, and
  [syft](https://github.com/anchore/syft) for the SBOMs.

## How a change is made

- The test comes first. Write the test that names the behaviour, watch it
  fail, then write the smallest code that passes it. This holds for small
  functions too.
- `make check` is green before every commit.
- The code builds and passes the tests on the go line of `go.mod`. Do not raise
  it, and do not add a toolchain line.
- The interface is English only. There is no translation layer.
- Comments are short and say why, not what the line does.
- The code is written here. Do not paste code from projects under a license
  other than MIT.

## Commits and pull requests

Commit messages and pull request titles are conventional commits, with the
part of azt as the scope: `feat(ui): ...`, `fix(azure): ...`,
`chore(ci): ...`.

One pull request is one change. Every push runs the checks on Linux, the tests
on Linux, macOS and Windows with the newest Go and on Linux with the oldest
one, govulncheck, and the whole release without publishing it. A pull request
needs all of them green.

## Releases

A maintainer pushes a `vX.Y.Z` tag. The tag publishes the release, the deb and
rpm packages and the Homebrew cask; a pull request changes nothing for that.
