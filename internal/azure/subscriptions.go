// Package azure reads Azure resources through Azure Resource Graph.
package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

// ResourceGraph is the part of *armresourcegraph.Client this package uses.
type ResourceGraph interface {
	Resources(ctx context.Context, query armresourcegraph.QueryRequest, options *armresourcegraph.ClientResourcesOptions) (armresourcegraph.ClientResourcesResponse, error)
}

type Subscription struct {
	ID    string `json:"subscriptionId"`
	Name  string `json:"name"`
	State string `json:"state"`
}

const subscriptionsQuery = `resourcecontainers
| where type =~ 'microsoft.resources/subscriptions'
| project subscriptionId, name, state = tostring(properties.state)
| order by name asc`

// Subscriptions returns every subscription the signed-in account can see.
func Subscriptions(ctx context.Context, graph ResourceGraph) ([]Subscription, error) {
	return query[Subscription](ctx, graph, subscriptionsQuery)
}
