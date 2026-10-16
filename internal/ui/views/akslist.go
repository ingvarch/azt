package views

import (
	"strconv"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

// NewAKSClusters lists the AKS clusters of every subscription.
func NewAKSClusters() View {
	return newList(listConfig[azure.Cluster]{
		key:   data.Key{Kind: data.AKSClusters},
		spec:  Spec{Kind: "resourcesByType", Type: aksType},
		title: KindName(aksType),
		noun:  "resources",
		empty: "No resources of this type.",
		columns: []column{
			{title: "Name"},
			{title: "Status", state: true},
			{title: "Kubernetes"},
			{title: "Nodes"},
			{title: "Tier"},
			{title: "Auto-upgrade"},
			{title: "Resource group"},
			{title: "Subscription"},
			{title: "Location"},
		},
		row:  clusterRow,
		open: func(c azure.Cluster) View { return NewDetails(c.SubscriptionID, c.Resource) },
	})
}

func clusterRow(c azure.Cluster) []string {
	status := join(stateSeparator, c.Power, notSucceeded(c.Provisioning))
	return []string{
		c.Name, orDash(status), orDash(first(c.CurrentVersion, c.Version)), strconv.Itoa(c.Nodes()), orDash(c.Tier),
		first(c.Channel, "none"), c.ResourceGroup, c.Subscription, c.Location,
	}
}
