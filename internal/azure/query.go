package azure

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

// query runs kql against the given subscriptions (none: the whole tenant),
// follows every page and decodes the rows into T.
func query[T any](ctx context.Context, graph ResourceGraph, kql string, subscriptions ...string) ([]T, error) {
	var scope []*string
	for _, id := range subscriptions {
		scope = append(scope, to.Ptr(id))
	}

	var rows []T
	var skipToken *string
	for {
		resp, err := graph.Resources(ctx, armresourcegraph.QueryRequest{
			Query:         to.Ptr(kql),
			Subscriptions: scope,
			Options: &armresourcegraph.QueryRequestOptions{
				ResultFormat: to.Ptr(armresourcegraph.ResultFormatObjectArray),
				SkipToken:    skipToken,
			},
		}, nil)
		if err != nil {
			return nil, err
		}

		page, err := decode[T](resp.Data)
		if err != nil {
			return nil, err
		}
		rows = append(rows, page...)

		if resp.SkipToken == nil || *resp.SkipToken == "" {
			return rows, nil
		}
		skipToken = resp.SkipToken
	}
}

// decode converts the SDK's untyped objectArray rows into T.
func decode[T any](data any) ([]T, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	var rows []T
	err = json.Unmarshal(raw, &rows)
	return rows, err
}

var kqlEscaper = strings.NewReplacer(`\`, `\\`, `'`, `\'`)

// kqlString returns s as a single-quoted KQL string literal.
func kqlString(s string) string {
	return "'" + kqlEscaper.Replace(s) + "'"
}
