// Package azure reads Azure resources through Azure Resource Graph.
package azure

import (
	"context"
	"encoding/json"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
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
	var subs []Subscription
	var skipToken *string
	for {
		resp, err := graph.Resources(ctx, armresourcegraph.QueryRequest{
			Query: to.Ptr(subscriptionsQuery),
			Options: &armresourcegraph.QueryRequestOptions{
				ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
				SkipToken:    skipToken,
			},
		}, nil)
		if err != nil {
			return nil, err
		}

		page, err := decode(resp.Data)
		if err != nil {
			return nil, err
		}
		subs = append(subs, page...)

		if resp.SkipToken == nil || *resp.SkipToken == "" {
			return subs, nil
		}
		skipToken = resp.SkipToken
	}
}

// decode converts the SDK's untyped objectArray rows into subscriptions.
func decode(data any) ([]Subscription, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var subs []Subscription
	err = json.Unmarshal(raw, &subs)
	return subs, err
}
