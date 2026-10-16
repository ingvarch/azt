package azure

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
)

func TestForbidden(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"403", &azcore.ResponseError{StatusCode: 403}, true},
		{"wrapped 403", fmt.Errorf("read: %w", &azcore.ResponseError{StatusCode: 403}), true},
		{"400", &azcore.ResponseError{StatusCode: 400}, false},
		{"plain error", errors.New("403"), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Forbidden(tt.err); got != tt.want {
				t.Errorf("Forbidden = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestErrorText(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"response error", &azcore.ResponseError{StatusCode: 400, ErrorCode: "BadRequest"}, "400 BadRequest"},
		{"wrapped", fmt.Errorf("read: %w", &azcore.ResponseError{StatusCode: 429, ErrorCode: "TooManyRequests"}), "429 TooManyRequests"},
		{"without a code", &azcore.ResponseError{StatusCode: 502}, "502"},
		{"plain error", errors.New("connection refused"), "connection refused"},
		{"dial error", &url.Error{Op: "Get", URL: "https://management.azure.com/subscriptions/s/x", Err: &net.OpError{Op: "dial", Net: "tcp", Err: errors.New("no such host")}}, "dial tcp: no such host"},
		{"timeout", fmt.Errorf("read: %w", &url.Error{Op: "Get", URL: "https://management.azure.com/subscriptions/s/x", Err: context.DeadlineExceeded}), "context deadline exceeded"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ErrorText(tt.err); got != tt.want {
				t.Errorf("ErrorText = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNextPeriod(t *testing.T) {
	for from, want := range map[string]string{"24h": "7d", "7d": "30d", "30d": "24h"} {
		if got := NextPeriod(from); got != want {
			t.Errorf("NextPeriod(%q) = %q, want %q", from, got, want)
		}
	}
}
