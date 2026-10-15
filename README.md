<div align="center">

# azt

**A terminal UI for Azure.**
Tabs, split panes and one session for every subscription you can see.

[![ci](https://github.com/ingvarch/azt/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/ingvarch/azt/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/ingvarch/azt)](https://github.com/ingvarch/azt/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/ingvarch/azt)](go.mod)
[![license](https://img.shields.io/github/license/ingvarch/azt)](LICENSE)

</div>

## What it does

- Walks from subscriptions to resource groups, resources and the full JSON of
  a resource: `enter` goes in, `esc` comes back.
- Lists every VM, AKS cluster, key vault, SQL server, storage account, web app,
  virtual network, managed identity or container registry across all your
  subscriptions in one table: `:vm`, `:aks`, `:kv` and the other commands.
- Splits the screen into panes and keeps several tabs, the way vim does with
  `ctrl+w`. Each pane has its own path; a split copies it.
- Shows an explorer tree of subscriptions, resource groups and resource types
  with counts.
- Filters any list as you type with `/`.
- Works with the mouse too: click to focus or select, double click to open,
  drag a border to resize, scroll with the wheel.
- Restores the tabs, panes and their paths the next time you start it.

azt reads through [Azure Resource Graph](https://learn.microsoft.com/azure/governance/resource-graph/overview),
so a list of thousands of resources across subscriptions is one query. It only
reads: it does not create, change or delete anything.

## Install

On macOS or Linux, with Homebrew:

```sh
brew install ingvarch/tap/azt
```

On Debian, Ubuntu, Fedora or RHEL, download the `.deb` or `.rpm` package from
the [releases](https://github.com/ingvarch/azt/releases) page and install it:

```sh
sudo apt install ./azt_*_amd64.deb
sudo dnf install ./azt-*.x86_64.rpm
```

For other systems (Linux, macOS, Windows and FreeBSD, on amd64 and ARM),
download an archive from the same page, or install with Go:

```sh
go install github.com/ingvarch/azt/cmd/azt@latest
```

To build from source:

```sh
make build && ./bin/azt
```

## Quick start

azt uses the account of the [Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli).
No configuration file is needed.

```sh
az login
azt
```

Press `?` for help, `:` for a command, `/` to filter and `q` to quit.

## Keys

| Key | What it does |
| --- | --- |
| `enter` / `esc` | open the selected item / go back |
| `j` `k`, `↑` `↓` | move |
| `/` | filter the list |
| `:` | command: `subs`, `rg`, `vm`, `aks`, `kv`, `sql`, `storage`, `app`, `vnet`, `mi`, `acr`, `type <type>`, `help`, `q` |
| `r` | refresh |
| `ctrl+w v` / `ctrl+w s` | split into left and right / top and bottom |
| `ctrl+w h j k l`, `ctrl+w` arrows | move to the neighbor pane or the explorer |
| `ctrl+w q`, `ctrl+w z`, `ctrl+w =` | close, zoom, equalize panes |
| `ctrl+w t`, `ctrl+w n p 1-9`, `ctrl+w ,` | new tab, switch tabs, rename the tab |
| `ctrl+w e` | show or hide the explorer |
| `?` | every key |

While azt runs, the terminal's own text selection needs Shift (Option in
iTerm2), because azt takes the mouse.

The session is saved to `$XDG_STATE_HOME/azt/session.json`, by default
`~/.local/state/azt/session.json`. Delete the file to start with one empty
tab.

## Contributing

How to build azt, run the checks and send a change is in
[CONTRIBUTING.md](CONTRIBUTING.md). Security problems are reported privately,
see [SECURITY.md](SECURITY.md).

## License

MIT, see [LICENSE](LICENSE). The release archives and packages also carry
`THIRD_PARTY_NOTICES`, the licenses of the modules azt is built from.

Microsoft and Azure are trademarks of the Microsoft group of companies. azt is
an independent project and is not affiliated with or endorsed by Microsoft.
