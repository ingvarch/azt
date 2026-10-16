package azure

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

const mixedCaseClusterID = "/subscriptions/S1/resourceGroups/RG/providers/Microsoft.ContainerService/managedClusters/Aks1"

func TestAlertRulesQuery(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil)}}

	if _, err := AlertRules(context.Background(), graph, "s1", mixedCaseClusterID); err != nil {
		t.Fatal(err)
	}

	if got := scope(graph.requests[0]); !reflect.DeepEqual(got, []string{"s1"}) {
		t.Errorf("scope = %v, want [s1]", got)
	}
	kql := *graph.requests[0].Query
	for _, want := range []string{
		"'microsoft.insights/metricalerts'",
		"'microsoft.insights/scheduledqueryrules'",
		"'microsoft.insights/activitylogalerts'",
		"'microsoft.alertsmanagement/prometheusrulegroups'",
		`contains '"` + strings.ToLower(mixedCaseClusterID) + `"'`,
		"tostring(properties.clusterName) =~ 'Aks1'",
		"ruleKind = case(",
		"tobool(properties.enabled)",
		"severity = iff(isnull(properties.severity), '', strcat('Sev', tostring(properties.severity)))",
		"type =~ 'microsoft.insights/metricalerts', 'metric'",
		"type =~ 'microsoft.insights/scheduledqueryrules', 'log'",
		"type =~ 'microsoft.insights/activitylogalerts', 'activity log'",
		"'Prometheus')",
		"order by name asc",
	} {
		if !strings.Contains(kql, want) {
			t.Errorf("query lacks %s:\n%s", want, kql)
		}
	}
	// Resource Graph fails the whole query with ParserFailure when a column is named kind.
	if strings.Contains(kql, " kind =") {
		t.Errorf("query names a column kind:\n%s", kql)
	}
}

func TestAlertRulesDecodeRows(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil,
		map[string]any{"name": "cpu", "ruleKind": "metric", "enabled": true, "severity": "Sev2"},
		map[string]any{"name": "audit", "ruleKind": "activity log", "enabled": false, "severity": ""},
		map[string]any{"name": "prom", "ruleKind": "Prometheus", "enabled": nil, "severity": ""},
	)}}

	got, err := AlertRules(context.Background(), graph, "s1", clusterID)
	if err != nil {
		t.Fatal(err)
	}

	want := []AlertRule{
		{Name: "cpu", Kind: "metric", Enabled: true, Severity: "Sev2"},
		{Name: "audit", Kind: "activity log"},
		{Name: "prom", Kind: "Prometheus"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("rules = %+v, want %+v", got, want)
	}
}

func TestAlertRulesReturnsGraphError(t *testing.T) {
	boom := errors.New("boom")

	if _, err := AlertRules(context.Background(), &fakeGraph{err: boom}, "s1", clusterID); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}

func TestAlertsQuery(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil)}}

	if _, err := Alerts(context.Background(), graph, "s1", mixedCaseClusterID); err != nil {
		t.Fatal(err)
	}

	if got := scope(graph.requests[0]); !reflect.DeepEqual(got, []string{"s1"}) {
		t.Errorf("scope = %v, want [s1]", got)
	}
	kql := *graph.requests[0].Query
	for _, want := range []string{
		"alertsmanagementresources",
		"type =~ 'microsoft.alertsmanagement/alerts'",
		"tolower(tostring(properties.essentials.targetResource)) == '" + strings.ToLower(mixedCaseClusterID) + "'",
		"alertRule = tostring(properties.essentials.alertRule)",
		"severity = tostring(properties.essentials.severity)",
		"condition = tostring(properties.essentials.monitorCondition)",
		"state = tostring(properties.essentials.alertState)",
		"started = todatetime(properties.essentials.startDateTime)",
		"order by started desc",
	} {
		if !strings.Contains(kql, want) {
			t.Errorf("query lacks %s:\n%s", want, kql)
		}
	}
}

func TestAlertsDecodeRows(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil,
		map[string]any{
			"alertRule": "/subscriptions/s1/resourceGroups/rg/providers/microsoft.insights/metricalerts/node-cpu",
			"severity":  "Sev1", "condition": "Fired", "state": "New",
			"started": "2026-10-15T10:30:00.1234567Z",
		},
		map[string]any{
			"alertRule": "plain-name",
			"severity":  "Sev3", "condition": "Resolved", "state": "Closed",
			"started": "2026-10-14T08:00:00Z",
		},
		map[string]any{"alertRule": nil, "severity": "Sev4", "condition": "Fired", "state": "New", "started": "2026-10-13T08:00:00Z"},
	)}}

	got, err := Alerts(context.Background(), graph, "s1", clusterID)
	if err != nil {
		t.Fatal(err)
	}

	want := []Alert{
		{Rule: "node-cpu", Severity: "Sev1", Condition: "Fired", State: "New",
			Started: time.Date(2026, 10, 15, 10, 30, 0, 123456700, time.UTC)},
		{Rule: "plain-name", Severity: "Sev3", Condition: "Resolved", State: "Closed",
			Started: time.Date(2026, 10, 14, 8, 0, 0, 0, time.UTC)},
		{Severity: "Sev4", Condition: "Fired", State: "New", Started: time.Date(2026, 10, 13, 8, 0, 0, 0, time.UTC)},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("alerts = %+v, want %+v", got, want)
	}
}

func TestAlertsRejectsBadTime(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil,
		map[string]any{"alertRule": "r", "started": "yesterday"},
	)}}

	if _, err := Alerts(context.Background(), graph, "s1", clusterID); err == nil {
		t.Error("err = nil, want a time parse error")
	}
}

func TestAlertsReturnsGraphError(t *testing.T) {
	boom := errors.New("boom")

	if _, err := Alerts(context.Background(), &fakeGraph{err: boom}, "s1", clusterID); !errors.Is(err, boom) {
		t.Errorf("err = %v, want %v", err, boom)
	}
}
