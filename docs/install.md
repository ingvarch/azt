# Install

How to install azt on each system. How to use it is in [usage.md](usage.md).

azt signs in with the account of the
[Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli), so
install that too and run `az login` once.

## Homebrew

On macOS or Linux:

```sh
brew install ingvarch/tap/azt
```

## Debian, Ubuntu, Fedora, RHEL

Download the `.deb` or `.rpm` package from the
[releases](https://github.com/ingvarch/azt/releases) page and install it:

```sh
sudo apt install ./azt_*_amd64.deb
sudo dnf install ./azt-*.x86_64.rpm
```

## Other systems

The [releases](https://github.com/ingvarch/azt/releases) page has an archive
for Linux, macOS, Windows and FreeBSD, on amd64 and ARM. Each archive holds
the binary, `LICENSE`, `README.md` and `THIRD_PARTY_NOTICES`, the licenses of
the modules azt is built from.

To check a download, follow
[Verifying a release](../SECURITY.md#verifying-a-release).

## Go

```sh
go install github.com/ingvarch/azt/cmd/azt@latest
```

## From source

```sh
make build && ./bin/azt
```

What the build needs is in [development.md](development.md).
