package azure

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrNotFound is returned when a resource does not exist.
var ErrNotFound = errors.New("resource not found")

// ResourceGroup is a resource group as Resource Graph returns it.
type ResourceGroup struct {
	Name           string `json:"name"`
	Location       string `json:"location"`
	State          string `json:"state"`
	SubscriptionID string `json:"subscriptionId"`
	Subscription   string `json:"subscription"` // name; only tenant-wide queries fill it
}

// Resource is an Azure resource as Resource Graph returns it.
type Resource struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Location       string `json:"location"`
	ResourceGroup  string `json:"resourceGroup"`
	SubscriptionID string `json:"subscriptionId"`
	Subscription   string `json:"subscription"` // name; only tenant-wide queries fill it
}

const resourceGroupsQuery = `resourcecontainers
| where type =~ 'microsoft.resources/subscriptions/resourcegroups'
| project name, location, state = tostring(properties.provisioningState), subscriptionId
| order by name asc`

// withSubscriptionName adds the subscription name to rows that carry subscriptionId.
const withSubscriptionName = `| join kind=leftouter (resourcecontainers | where type =~ 'microsoft.resources/subscriptions' | project subscriptionId, subscription = name) on subscriptionId`

const allResourceGroupsQuery = `resourcecontainers
| where type =~ 'microsoft.resources/subscriptions/resourcegroups'
` + withSubscriptionName + `
| project name, location, state = tostring(properties.provisioningState), subscriptionId, subscription
| order by subscription asc, name asc`

// AllResourceGroups returns the resource groups of every subscription.
func AllResourceGroups(ctx context.Context, graph ResourceGraph) ([]ResourceGroup, error) {
	return query[ResourceGroup](ctx, graph, allResourceGroupsQuery)
}

// ResourceGroups returns the resource groups of one subscription.
func ResourceGroups(ctx context.Context, graph ResourceGraph, subscriptionID string) ([]ResourceGroup, error) {
	return query[ResourceGroup](ctx, graph, resourceGroupsQuery, subscriptionID)
}

// Resources returns the resources of one resource group.
func Resources(ctx context.Context, graph ResourceGraph, subscriptionID, resourceGroup string) ([]Resource, error) {
	kql := `resources
| where resourceGroup =~ ` + kqlString(resourceGroup) + `
| project id, name, type, location, resourceGroup, subscriptionId
| order by name asc`
	return query[Resource](ctx, graph, kql, subscriptionID)
}

// ResourcesOfType returns the resources of the given types in every subscription.
func ResourcesOfType(ctx context.Context, graph ResourceGraph, resourceTypes []string) ([]Resource, error) {
	quoted := make([]string, len(resourceTypes))
	for i, t := range resourceTypes {
		quoted[i] = kqlString(t)
	}
	kql := `resources
| where type in~ (` + strings.Join(quoted, ", ") + `)
` + withSubscriptionName + `
| project id, name, type, location, resourceGroup, subscriptionId, subscription
| order by name asc`
	return query[Resource](ctx, graph, kql)
}

// ResourceDetails returns every property of one resource, with the subscription name.
func ResourceDetails(ctx context.Context, graph ResourceGraph, subscriptionID, resourceID string) (map[string]any, error) {
	// The join repeats its key as subscriptionId1; the row needs it once.
	kql := "resources\n| where id =~ " + kqlString(resourceID) + "\n" + withSubscriptionName + "\n| project-away subscriptionId1"
	rows, err := query[map[string]any](ctx, graph, kql, subscriptionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, resourceID)
	}
	return rows[0], nil
}
