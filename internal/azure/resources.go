package azure

import (
	"context"
	"errors"
	"fmt"
)

// ErrNotFound is returned when a resource does not exist.
var ErrNotFound = errors.New("resource not found")

type ResourceGroup struct {
	Name     string `json:"name"`
	Location string `json:"location"`
	State    string `json:"state"`
}

type Resource struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Location string `json:"location"`
}

const resourceGroupsQuery = `resourcecontainers
| where type =~ 'microsoft.resources/subscriptions/resourcegroups'
| project name, location, state = tostring(properties.provisioningState)
| order by name asc`

// ResourceGroups returns the resource groups of one subscription.
func ResourceGroups(ctx context.Context, graph ResourceGraph, subscriptionID string) ([]ResourceGroup, error) {
	return query[ResourceGroup](ctx, graph, resourceGroupsQuery, subscriptionID)
}

// Resources returns the resources of one resource group.
func Resources(ctx context.Context, graph ResourceGraph, subscriptionID, resourceGroup string) ([]Resource, error) {
	kql := `resources
| where resourceGroup =~ ` + kqlString(resourceGroup) + `
| project id, name, type, location
| order by name asc`
	return query[Resource](ctx, graph, kql, subscriptionID)
}

// ResourceDetails returns every property of one resource.
func ResourceDetails(ctx context.Context, graph ResourceGraph, subscriptionID, resourceID string) (map[string]any, error) {
	kql := "resources\n| where id =~ " + kqlString(resourceID)
	rows, err := query[map[string]any](ctx, graph, kql, subscriptionID)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, resourceID)
	}
	return rows[0], nil
}
