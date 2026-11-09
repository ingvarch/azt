# Usage

What azt shows, the keys and the mouse, the session file and what azt reads.
How to install it is in [install.md](install.md).

## Start

azt uses the account of the
[Azure CLI](https://learn.microsoft.com/cli/azure/install-azure-cli). No
configuration file is needed.

```sh
az login
azt
```

Press `?` for every key, `:` for a command, `/` to filter and `q` to quit.

## Views

- Walk from subscriptions to resource groups, resources and the full JSON of
  a resource: `enter` goes in, `esc` comes back.
- See every resource of one kind across all your subscriptions in one table:
  virtual machines (`:vm`), AKS clusters (`:aks`), key vaults (`:kv`), SQL
  servers (`:sql`), PostgreSQL servers (`:pg`), DocumentDB clusters
  (`:docdb`), AI Foundry resources and projects (`:foundry`), Front Door and
  CDN profiles (`:afd`), storage accounts (`:storage`), web apps (`:app`),
  virtual networks (`:vnet`), managed identities (`:mi`), container
  registries (`:acr`), network security groups (`:nsg`), and public and
  private DNS zones (`:dns`). `:type <type>` lists any other resource type.
- `:vm` shows each VM's status, size, OS and IPs. `:aks` shows each cluster's
  status, Kubernetes version, nodes, tier and auto-upgrade channel.
- A resource has tabs; `tab` and `shift-tab` switch them. The Monitoring tab
  of an AKS cluster shows how its monitoring is set up and how its nodes,
  pods and control plane did over the last 24 hours, 7 or 30 days. `p`
  switches the period.
- The explorer on the left is a tree of subscriptions, resource groups and
  resource types with counts.
- `/` filters any list as you type.

## Tabs and panes

azt splits the screen into panes and keeps several tabs, the way vim does
with `ctrl-w`. Each pane has its own path, and a split copies it. The title of
a list names its kind and the number of rows, such as `AKS clusters [2]`.

Each tab can show one subscription, chosen with `0-9` (`0` is all); the
numbers are shown in `:subs`.

## Keys

| Key | What it does |
| --- | --- |
| `enter` / `esc` | open the selected item / go back |
| `j` `k`, `↑` `↓` | move |
| `tab` / `shift-tab` | next / previous tab of a resource |
| `/` | filter the list: `enter` keeps the filter, `esc` clears it |
| `:` | command: `subs`, `rg`, `vm`, `aks`, `kv`, `sql`, `pg`, `docdb`, `foundry`, `afd`, `storage`, `app`, `vnet`, `mi`, `acr`, `nsg`, `dns`, `type <type>`, `help`, `q` |
| `0-9` | subscription of the tab: `0` all, `1`-`9` as numbered in `:subs` |
| `p` | period of the Monitoring tab |
| `r` | refresh |
| `y` / `Y` | copy the ID / name of the selected item |
| `?` | every key |
| `q`, `ctrl-c` | quit; in the tab dialog `ctrl-c` clears the field |
| `ctrl-w v` / `ctrl-w s` | split into left and right / top and bottom |
| `ctrl-w h j k l`, `ctrl-w` arrows | move to the neighbor pane or the explorer |
| `ctrl-w q`, `ctrl-w z`, `ctrl-w =` | close, zoom, equalize panes |
| `ctrl-w < > + -` | resize the pane |
| `ctrl-w t`, `ctrl-w n p 1-9`, `ctrl-w ,` | new tab, switch tabs, rename the tab |
| `ctrl-w e` | show or hide the explorer |
| `l` `h`, `→` `←` | in the explorer: expand / collapse |
| `ctrl-w < >`, `ctrl-w =` | with the explorer focused: make it narrower or wider, or reset it to the default width |

In the command dialog, `↑` `↓` choose a command and `tab` completes it.

## Mouse

Click to focus or select, double click to open, drag a border to resize panes
or the explorer, scroll with the wheel.

While azt runs, the terminal's own text selection needs Shift (Option in
iTerm2), because azt takes the mouse.

## Session

azt restores the tabs, panes and their paths the next time you start it. The
session is saved to `$XDG_STATE_HOME/azt/session.json`, by default
`~/.local/state/azt/session.json`. Delete the file to start with one empty
tab.

## What azt reads

azt reads through
[Azure Resource Graph](https://learn.microsoft.com/azure/governance/resource-graph/overview),
so a list of thousands of resources across subscriptions is one query. The
Monitoring tab also reads Azure Monitor metrics and diagnostic settings
through Azure Resource Manager, with GET requests only. azt does not create,
change or delete anything. Reader on a cluster is enough to see its metrics.
