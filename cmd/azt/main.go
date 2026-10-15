// Command azt is a terminal UI for Azure.
package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"

	"github.com/ingvarch/azt/internal/session"
	"github.com/ingvarch/azt/internal/ui"
	"github.com/ingvarch/azt/internal/ui/data"
	"github.com/ingvarch/azt/internal/ui/views"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "azt:", err)
		os.Exit(1)
	}
}

func run() error {
	// Uses the account from `az login`.
	cred, err := azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return err
	}
	graph, err := armresourcegraph.NewClient(cred, nil)
	if err != nil {
		return err
	}

	// Cancels a request still running when the user quits.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fetch := data.NewFetcher(ctx, graph)
	// Without a state directory azt still runs, but forgets the layout.
	var m ui.Model
	path, pathErr := session.DefaultPath()
	if pathErr != nil {
		m = ui.New(views.NewSubscriptions, fetch)
	} else {
		m = ui.LoadOrNew(path, views.NewSubscriptions, fetch)
	}
	final, err := tea.NewProgram(m).Run()
	if err != nil || path == "" {
		return err
	}
	if err := session.Save(path, final.(ui.Model).Session()); err != nil {
		return fmt.Errorf("save session: %w", err)
	}
	return nil
}
