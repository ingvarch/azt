# Contributing to azt

Bug reports, ideas and pull requests are welcome. Everyone who takes part
follows the [Code of Conduct](CODE_OF_CONDUCT.md). A security problem is
reported privately, not in an issue: see [SECURITY.md](SECURITY.md).

## Report a bug or ask for a feature

Open an issue with one of the
[issue templates](https://github.com/ingvarch/azt/issues/new/choose). A bug
report needs the azt and Azure CLI versions and the steps that show the
problem.

## Before you write code

For anything bigger than a small fix, open an issue first, so the shape of the
change is agreed before the work is done.

## Build and run

What you need, how to run azt and the checks, and where the code lives are in
[docs/development.md](docs/development.md).

## How a change is made

- Work on a branch from `main` named `<type>/<short-name>`, such as
  `fix/vm-ip-column`.
- The test comes first. Write the test that names the behaviour, watch it
  fail, then write the smallest code that passes it. This holds for small
  functions too.
- `make check` is green before every commit.
- The code builds and passes the tests on the go line of `go.mod`. Do not raise
  it, and do not add a toolchain line.
- The interface is English only. There is no translation layer.
- A change that users see updates [docs/usage.md](docs/usage.md) too.
- Comments are short and say why, not what the line does.
- The code is written here. Do not paste code from projects under a license
  other than MIT.

## Commits and pull requests

Commit messages and pull request titles are conventional commits, with the
part of azt as the scope: `feat(ui): ...`, `fix(azure): ...`,
`chore(ci): ...`.

One pull request is one change. Fill in the pull request template and link the
issue it closes with `Closes #N`. Pull requests are merged with rebase or
squash, so `main` keeps a linear history.

Every push runs the checks on Linux, the tests on Linux, macOS and Windows with
the newest Go and on Linux with the oldest one, govulncheck, and the whole
release without publishing it. A pull request needs all of them green.

The rules for agents working in this repository are in [AGENTS.md](AGENTS.md).

## Releases

A maintainer pushes a `vX.Y.Z` tag. The tag publishes the release, the deb and
rpm packages and the Homebrew cask; a pull request changes nothing for that.
The steps are in [docs/releasing.md](docs/releasing.md).

## License

By contributing, you agree that your contributions are licensed under the
[MIT License](LICENSE).
