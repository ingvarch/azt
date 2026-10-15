package ui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/ingvarch/azt/internal/ui/views"
)

type commandKind int

const (
	cmdNone commandKind = iota
	cmdView
	cmdHelp
	cmdQuit
)

// command is a parsed palette input. view is set for cmdView.
type command struct {
	kind commandKind
	view views.View
}

// commandInfo is a palette command and what it does.
type commandInfo struct{ name, desc string }

// commandTable lists the palette commands, in the order of the dialog.
var commandTable = buildCommandTable()

func buildCommandTable() []commandInfo {
	table := []commandInfo{
		{"subs", "subscriptions"},
		{"rg", "resource groups in every subscription"},
	}
	for _, c := range views.TypeCommands() {
		table = append(table, commandInfo{c.Name, c.Desc})
	}
	return append(table,
		commandInfo{"type", "resources of any type: type <resource type>"},
		commandInfo{"help", "keys and commands"},
		commandInfo{"q", "quit"},
	)
}

// typeCommand returns the resource types the command called name lists.
func typeCommand(name string) ([]string, bool) {
	for _, c := range views.TypeCommands() {
		if c.Name == name {
			return c.Types, true
		}
	}
	return nil, false
}

// parseCommand turns palette input into a command; the first word names it.
func parseCommand(input string) (command, error) {
	words := strings.Fields(input)
	if len(words) == 0 {
		return command{}, nil
	}
	name := strings.ToLower(words[0])
	if types, ok := typeCommand(name); ok {
		return command{cmdView, views.NewResourcesByType(types...)}, nil
	}
	switch name {
	case "subs":
		return command{cmdView, views.NewSubscriptions()}, nil
	case "rg":
		return command{cmdView, views.NewAllResourceGroups()}, nil
	case "type":
		if len(words) < 2 {
			return command{}, errors.New("usage: type <resource type>")
		}
		return command{cmdView, views.NewResourcesByType(words[1])}, nil
	case "help":
		return command{kind: cmdHelp}, nil
	case "q", "quit":
		return command{kind: cmdQuit}, nil
	}
	return command{}, fmt.Errorf("unknown command: %s", words[0])
}

// firstWord splits input into its first word and the rest, which keeps its leading space.
func firstWord(input string) (word, rest string) {
	input = strings.TrimLeft(input, " ")
	i := strings.IndexByte(input, ' ')
	if i < 0 {
		return input, ""
	}
	return input[:i], input[i:]
}

// matchingCommands returns the commands that start with the first word of input.
func matchingCommands(input string) []string {
	word, _ := firstWord(input)
	word = strings.ToLower(word)
	var out []string
	for _, c := range commandTable {
		if strings.HasPrefix(c.name, word) {
			out = append(out, c.name)
		}
	}
	return out
}
