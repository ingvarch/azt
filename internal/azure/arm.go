package azure

import (
	"context"
	"net/http"
	"net/url"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"

	"github.com/ingvarch/azt/internal/version"
)

// ARM sends GET requests to Azure Resource Manager; azt sends nothing else.
type ARM interface {
	// Get returns the body of a GET of path, such as a resource ID plus a suffix, with query.
	Get(ctx context.Context, path string, query url.Values) ([]byte, error)
}

type armClient struct {
	client *arm.Client
}

// NewARM returns an ARM client that signs its requests with cred.
func NewARM(cred azcore.TokenCredential, options *arm.ClientOptions) (ARM, error) {
	var opts arm.ClientOptions
	if options != nil {
		opts = *options
	}
	// Registration would answer a 409 with a POST; telemetry would reject the version "dev".
	opts.DisableRPRegistration = true
	opts.Telemetry.Disabled = true

	client, err := arm.NewClient("azt", version.Version, cred, &opts)
	if err != nil {
		return nil, err
	}
	return armClient{client: client}, nil
}

func (c armClient) Get(ctx context.Context, path string, query url.Values) ([]byte, error) {
	req, err := runtime.NewRequest(ctx, http.MethodGet, runtime.JoinPaths(c.client.Endpoint(), path))
	if err != nil {
		return nil, err
	}
	req.Raw().URL.RawQuery = query.Encode()
	req.Raw().Header.Set("Accept", "application/json")

	resp, err := c.client.Pipeline().Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, runtime.NewResponseError(resp)
	}
	return runtime.Payload(resp)
}
