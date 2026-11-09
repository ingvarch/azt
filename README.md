# azt

[![ci](https://github.com/ingvarch/azt/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/ingvarch/azt/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/ingvarch/azt)](https://github.com/ingvarch/azt/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/ingvarch/azt?logo=go&logoColor=white)](go.mod)
[![license](https://img.shields.io/github/license/ingvarch/azt)](LICENSE)

azt is a terminal UI for Azure, for people who work across many
subscriptions. It shows the resources of every subscription you can see in
one table, opens any resource down to its JSON and shows how an AKS cluster
did over the last 30 days, in tabs and split panes you drive from the keyboard.
azt only reads: it does not create, change or delete anything. Igor Churmeev
makes it.

## Features

- One table of every VM, AKS cluster, key vault, database, storage account,
  network and more across all your subscriptions: `:vm`, `:aks`, `:kv`.
- Walk from subscriptions to resource groups, resources and a resource's full
  JSON.
- AKS monitoring: how it is set up and how the nodes, pods and control plane
  did over 24 hours, 7 or 30 days.
- Tabs and split panes with vim's `ctrl-w` keys; each tab can show one
  subscription, chosen with `0-9`.
- An explorer tree of subscriptions, resource groups and resource types with
  counts.
- Filter any list as you type, copy an ID or a name with `y` and `Y`, or use
  the mouse.
- The tabs, panes and their paths come back the next time you start it.

## Usage

```sh
brew install ingvarch/tap/azt
az login
azt
```

Press `?` for every key. Packages for Debian and Fedora, archives for other
systems and `go install` are in [docs/install.md](docs/install.md).

## Quick start

You need Go and the Azure CLI, see [docs/development.md](docs/development.md).

```sh
git clone https://github.com/ingvarch/azt.git
cd azt
make test
az login
make run
```

## Documentation

- [Install](docs/install.md): Homebrew, deb and rpm packages, archives,
  `go install`, from source.
- [Usage](docs/usage.md): views, keys, mouse, the session file, what azt
  reads.
- [Development](docs/development.md): prerequisites, run, checks, project
  structure.
- [Releasing](docs/releasing.md): what a release publishes, how to make one,
  secrets, failures.

## License

MIT, see [LICENSE](LICENSE). The release archives and packages also carry
`THIRD_PARTY_NOTICES`, the licenses of the modules azt is built from.

Microsoft and Azure are trademarks of the Microsoft group of companies. azt is
an independent project and is not affiliated with or endorsed by Microsoft.

## Contributing

How to propose a change is in [CONTRIBUTING.md](CONTRIBUTING.md). Security
problems are reported privately, see [SECURITY.md](SECURITY.md).
