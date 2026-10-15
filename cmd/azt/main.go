// Command azt is a terminal UI for Azure.
package main

import (
	"context"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui"
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

	load := func() ([]azure.Subscription, error) { return azure.Subscriptions(ctx, graph) }
	_, err = tea.NewProgram(ui.New(load)).Run()
	return err
}
