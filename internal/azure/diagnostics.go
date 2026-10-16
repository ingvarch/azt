package azure

import (
	"cmp"
	"context"
	"encoding/json"
	"net/url"
)

// DiagnosticSetting is where one diagnostic setting sends a resource's logs and metrics.
type DiagnosticSetting struct {
	Name         string
	Logs         []string // enabled log categories or category groups, in order
	Metrics      bool     // whether a metrics category is enabled
	Destinations []string // "Log Analytics", "Storage", "Event Hubs", "Partner", in this order
}

// DiagnosticSettings reads the diagnostic settings of the resource with the given ID.
func DiagnosticSettings(ctx context.Context, arm ARM, resourceID string) ([]DiagnosticSetting, error) {
	body, err := arm.Get(ctx, resourceID+"/providers/Microsoft.Insights/diagnosticSettings",
		url.Values{"api-version": {"2021-05-01-preview"}})
	if err != nil {
		return nil, err
	}
	var resp struct {
		Value []struct {
			Name       string
			Properties struct {
				Logs []struct {
					Category      string
					CategoryGroup string
					Enabled       bool
				}
				Metrics []struct{ Enabled bool }

				WorkspaceID                 string `json:"workspaceId"`
				StorageAccountID            string `json:"storageAccountId"`
				EventHubAuthorizationRuleID string `json:"eventHubAuthorizationRuleId"`
				MarketplacePartnerID        string `json:"marketplacePartnerId"`
			}
		}
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	settings := make([]DiagnosticSetting, len(resp.Value))
	for i, v := range resp.Value {
		p := v.Properties
		s := DiagnosticSetting{Name: v.Name}
		for _, l := range p.Logs {
			if !l.Enabled {
				continue
			}
			s.Logs = append(s.Logs, cmp.Or(l.Category, l.CategoryGroup))
		}
		for _, m := range p.Metrics {
			s.Metrics = s.Metrics || m.Enabled
		}
		for _, d := range []struct{ name, id string }{
			{"Log Analytics", p.WorkspaceID},
			{"Storage", p.StorageAccountID},
			{"Event Hubs", p.EventHubAuthorizationRuleID},
			{"Partner", p.MarketplacePartnerID},
		} {
			if d.id != "" {
				s.Destinations = append(s.Destinations, d.name)
			}
		}
		settings[i] = s
	}
	return settings, nil
}
