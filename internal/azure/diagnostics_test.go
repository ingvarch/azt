package azure

import (
	"context"
	"errors"
	"net/url"
	"reflect"
	"testing"
)

func TestDiagnosticSettingsRequest(t *testing.T) {
	arm := &fakeARM{body: `{"value":[]}`}

	if _, err := DiagnosticSettings(context.Background(), arm, clusterID); err != nil {
		t.Fatal(err)
	}

	if want := []string{clusterID + "/providers/Microsoft.Insights/diagnosticSettings"}; !reflect.DeepEqual(arm.paths, want) {
		t.Errorf("paths = %v, want %v", arm.paths, want)
	}
	if want := (url.Values{"api-version": {"2021-05-01-preview"}}); !reflect.DeepEqual(arm.query[0], want) {
		t.Errorf("query = %v, want %v", arm.query[0], want)
	}
}

func TestDiagnosticSettingsParse(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []DiagnosticSetting
	}{
		{"empty value", `{"value":[]}`, []DiagnosticSetting{}},
		{
			"categories and groups, disabled left out",
			`{"value":[{"name":"d1","properties":{
				"logs":[{"category":"kube-audit","enabled":true},{"categoryGroup":"allLogs","enabled":true},{"category":"cluster-autoscaler","enabled":false},{"categoryGroup":"audit","enabled":false}],
				"metrics":[{"category":"AllMetrics","enabled":false}]}}]}`,
			[]DiagnosticSetting{{Name: "d1", Logs: []string{"kube-audit", "allLogs"}}},
		},
		{
			"metrics enabled",
			`{"value":[{"name":"d1","properties":{"metrics":[{"category":"AllMetrics","enabled":true}]}}]}`,
			[]DiagnosticSetting{{Name: "d1", Metrics: true}},
		},
		{
			"every destination in order",
			`{"value":[{"name":"d1","properties":{
				"marketplacePartnerId":"p","eventHubAuthorizationRuleId":"e","storageAccountId":"s","workspaceId":"w"}}]}`,
			[]DiagnosticSetting{{Name: "d1", Destinations: []string{"Log Analytics", "Storage", "Event Hubs", "Partner"}}},
		},
		{
			"null and empty destinations are none",
			`{"value":[{"name":"d1","properties":{"workspaceId":"","storageAccountId":null,"eventHubAuthorizationRuleId":null,"marketplacePartnerId":null}}]}`,
			[]DiagnosticSetting{{Name: "d1"}},
		},
		{
			"two settings keep their order",
			`{"value":[{"name":"b","properties":{"storageAccountId":"s"}},{"name":"a","properties":{"workspaceId":"w"}}]}`,
			[]DiagnosticSetting{
				{Name: "b", Destinations: []string{"Storage"}},
				{Name: "a", Destinations: []string{"Log Analytics"}},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DiagnosticSettings(context.Background(), &fakeARM{body: tt.body}, clusterID)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("settings = %#v, want %#v", got, tt.want)
			}
		})
	}
}

func TestDiagnosticSettingsReturnsARMError(t *testing.T) {
	boom := errors.New("boom")

	if _, err := DiagnosticSettings(context.Background(), &fakeARM{err: boom}, clusterID); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}

func TestDiagnosticSettingsRejectsBadBody(t *testing.T) {
	if _, err := DiagnosticSettings(context.Background(), &fakeARM{body: `nope`}, clusterID); err == nil {
		t.Error("err = nil, want a decode error")
	}
}
