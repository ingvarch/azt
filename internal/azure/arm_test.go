package azure

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/arm"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
)

type fakeCredential struct{}

func (fakeCredential) GetToken(context.Context, policy.TokenRequestOptions) (azcore.AccessToken, error) {
	return azcore.AccessToken{Token: "tok", ExpiresOn: time.Now().Add(time.Hour)}, nil
}

// fakeTransport records every request and answers with reply.
type fakeTransport struct {
	mu       sync.Mutex
	requests []*http.Request
	reply    func(*http.Request) (int, string)
}

func (f *fakeTransport) Do(r *http.Request) (*http.Response, error) {
	f.mu.Lock()
	f.requests = append(f.requests, r)
	f.mu.Unlock()
	status, body := f.reply(r)
	return &http.Response{
		StatusCode: status,
		Header:     http.Header{"Content-Type": {"application/json"}},
		Body:       io.NopCloser(strings.NewReader(body)),
		Request:    r,
	}, nil
}

func replyWith(status int, body string) func(*http.Request) (int, string) {
	return func(*http.Request) (int, string) { return status, body }
}

func newTestARM(t *testing.T, reply func(*http.Request) (int, string)) (ARM, *fakeTransport) {
	t.Helper()
	transport := &fakeTransport{reply: reply}
	client, err := NewARM(fakeCredential{}, &arm.ClientOptions{ClientOptions: policy.ClientOptions{
		Transport: transport,
		Retry:     policy.RetryOptions{MaxRetries: -1},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return client, transport
}

func TestARMGetSendsSignedGET(t *testing.T) {
	client, transport := newTestARM(t, replyWith(http.StatusOK, `{}`))

	query := url.Values{"api-version": {"2024-02-01"}, "metricnames": {"a,b"}}
	if _, err := client.Get(context.Background(), "/subscriptions/s1/x", query); err != nil {
		t.Fatal(err)
	}

	if len(transport.requests) != 1 {
		t.Fatalf("%d requests, want 1", len(transport.requests))
	}
	req := transport.requests[0]
	if req.Method != http.MethodGet {
		t.Errorf("method = %s, want GET", req.Method)
	}
	if got, want := req.URL.String(), "https://management.azure.com/subscriptions/s1/x?"+query.Encode(); got != want {
		t.Errorf("url = %s, want %s", got, want)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer tok" {
		t.Errorf("Authorization = %q, want Bearer tok", got)
	}
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q, want application/json", got)
	}
}

func TestARMGetReturnsBody(t *testing.T) {
	client, _ := newTestARM(t, replyWith(http.StatusOK, `{"value":[]}`))

	got, err := client.Get(context.Background(), "/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"value":[]}` {
		t.Errorf("body = %s", got)
	}
}

func TestARMGetEncodesFilterKey(t *testing.T) {
	client, transport := newTestARM(t, replyWith(http.StatusOK, `{}`))

	query := url.Values{"$filter": {"nodepool eq '*'"}}
	if _, err := client.Get(context.Background(), "/x", query); err != nil {
		t.Fatal(err)
	}
	if got := transport.requests[0].URL.RawQuery; !strings.HasPrefix(got, "%24filter=") {
		t.Errorf("query = %s, want it to start with %%24filter=", got)
	}
}

func TestARMGetErrors(t *testing.T) {
	tests := []struct {
		name   string
		status int
		body   string
		want   string
	}{
		{"arm form", http.StatusForbidden, `{"error":{"code":"AuthorizationFailed","message":"no access"}}`, "AuthorizationFailed"},
		{"insights form", http.StatusBadRequest, `{"code":"BadRequest","message":"bad filter"}`, "BadRequest"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newTestARM(t, replyWith(tt.status, tt.body))

			_, err := client.Get(context.Background(), "/x", nil)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to contain %s", err, tt.want)
			}
			var re *azcore.ResponseError
			if !errors.As(err, &re) || re.ErrorCode != tt.want {
				t.Errorf("err = %v, want a *azcore.ResponseError with code %s", err, tt.want)
			}
		})
	}
}

func TestARMGetNeverRegistersProvider(t *testing.T) {
	body := `{"error":{"code":"MissingSubscriptionRegistration","message":"register"}}`
	client, transport := newTestARM(t, replyWith(http.StatusConflict, body))

	_, err := client.Get(context.Background(), "/subscriptions/s1/resourceGroups/rg/providers/Microsoft.ContainerService/managedClusters/aks1", nil)
	if err == nil || !strings.Contains(err.Error(), "MissingSubscriptionRegistration") {
		t.Errorf("err = %v, want MissingSubscriptionRegistration", err)
	}
	if len(transport.requests) != 1 {
		t.Errorf("%d requests, want 1", len(transport.requests))
	}
}

func TestNewARMKeepsCallerOptions(t *testing.T) {
	opts := &arm.ClientOptions{}
	opts.Retry.MaxRetries = -1
	opts.Transport = &fakeTransport{reply: replyWith(http.StatusOK, `{}`)}
	if _, err := NewARM(fakeCredential{}, opts); err != nil {
		t.Fatal(err)
	}
	if opts.DisableRPRegistration || opts.Telemetry.Disabled {
		t.Error("NewARM changed the caller's options")
	}
}
