package azure

import (
	"context"
	"strings"
	"time"
)

// AlertRule is an alert rule that watches a resource.
type AlertRule struct {
	Name     string `json:"name"`
	Kind     string `json:"ruleKind"` // "metric", "log", "activity log", "Prometheus"
	Enabled  bool   `json:"enabled"`
	Severity string `json:"severity"` // "Sev0".."Sev4", empty when the rule has none
}

// AlertRules returns the alert rules whose scopes hold the resource, and the
// Prometheus rule groups of a cluster with the resource's name.
func AlertRules(ctx context.Context, graph ResourceGraph, subscriptionID, resourceID string) ([]AlertRule, error) {
	// Quoting the ID keeps /aks1 from matching /aks12. Resource Graph rejects a column named kind.
	kql := `resources
| where type in~ ('microsoft.insights/metricalerts', 'microsoft.insights/scheduledqueryrules', 'microsoft.insights/activitylogalerts', 'microsoft.alertsmanagement/prometheusrulegroups')
| where tolower(tostring(properties.scopes)) contains ` + kqlString(`"`+strings.ToLower(resourceID)+`"`) + `
    or (type =~ 'microsoft.alertsmanagement/prometheusrulegroups' and tostring(properties.clusterName) =~ ` + kqlString(lastSegment(resourceID)) + `)
| project name,
    ruleKind = case(
        type =~ 'microsoft.insights/metricalerts', 'metric',
        type =~ 'microsoft.insights/scheduledqueryrules', 'log',
        type =~ 'microsoft.insights/activitylogalerts', 'activity log',
        'Prometheus'),
    enabled = tobool(properties.enabled),
    severity = iff(isnull(properties.severity), '', strcat('Sev', tostring(properties.severity)))
| order by name asc`
	return query[AlertRule](ctx, graph, kql, subscriptionID)
}

// Alert is an alert that fired on a resource in the last 30 days.
type Alert struct {
	Rule      string // the rule's name: the last segment of essentials.alertRule
	Severity  string // "Sev0".."Sev4"
	Condition string // "Fired" or "Resolved"
	State     string // "New", "Acknowledged", "Closed"
	Started   time.Time
}

// Alerts returns the alerts on the resource, the newest first.
func Alerts(ctx context.Context, graph ResourceGraph, subscriptionID, resourceID string) ([]Alert, error) {
	kql := `alertsmanagementresources
| where type =~ 'microsoft.alertsmanagement/alerts'
| where tolower(tostring(properties.essentials.targetResource)) == ` + kqlString(strings.ToLower(resourceID)) + `
| project alertRule = tostring(properties.essentials.alertRule),
    severity = tostring(properties.essentials.severity),
    condition = tostring(properties.essentials.monitorCondition),
    state = tostring(properties.essentials.alertState),
    started = todatetime(properties.essentials.startDateTime)
| order by started desc`
	rows, err := query[struct {
		AlertRule string `json:"alertRule"`
		Severity  string `json:"severity"`
		Condition string `json:"condition"`
		State     string `json:"state"`
		Started   string `json:"started"`
	}](ctx, graph, kql, subscriptionID)
	if err != nil {
		return nil, err
	}

	alerts := make([]Alert, len(rows))
	for i, r := range rows {
		started, err := time.Parse(time.RFC3339, r.Started)
		if err != nil {
			return nil, err
		}
		alerts[i] = Alert{
			Rule:      lastSegment(r.AlertRule),
			Severity:  r.Severity,
			Condition: r.Condition,
			State:     r.State,
			Started:   started,
		}
	}
	return alerts, nil
}

// lastSegment is the part of a resource ID after the last slash.
func lastSegment(s string) string { return s[strings.LastIndex(s, "/")+1:] }
