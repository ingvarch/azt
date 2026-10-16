package azure

import "context"

// Cluster is an AKS cluster as the cluster list query returns it.
type Cluster struct {
	Resource
	Power          string `json:"power"`
	Provisioning   string `json:"provisioning"`
	CurrentVersion string `json:"currentVersion"`
	Version        string `json:"version"`
	Pools          []Pool `json:"pools"`
	Karpenter      int    `json:"karpenter"` // VMs in the node resource group with the Karpenter tag
	Tier           string `json:"tier"`
	Channel        string `json:"channel"` // upgrade channel
}

// Pool is a node pool of a cluster, with its current node count.
type Pool struct {
	Count int `json:"count"`
}

// Nodes is the pools' node counts plus the Karpenter nodes, which the pools do not count.
func (c Cluster) Nodes() int {
	n := c.Karpenter
	for _, p := range c.Pools {
		n += p.Count
	}
	return n
}

// The pools do not count Karpenter nodes, so the query adds the number of VMs
// in the node resource group that carry the Karpenter tag. The nodes are summed
// in Go: Resource Graph returns only the first page of a top-level summarize.
const clustersQuery = `resources
| where type =~ 'microsoft.containerservice/managedclusters'
| extend nodeRg = tolower(tostring(properties.nodeResourceGroup))
| join kind=leftouter (
    resources
    | where type =~ 'microsoft.compute/virtualmachines' and isnotempty(tostring(tags['karpenter.sh_nodepool']))
    | summarize karpenter = count() by subscriptionId, nodeRg = tolower(resourceGroup)
  ) on subscriptionId, nodeRg
` + withSubscriptionName + `
| project id, name, type, location, resourceGroup, subscriptionId, subscription,
    power = tostring(properties.powerState.code),
    provisioning = tostring(properties.provisioningState),
    currentVersion = tostring(properties.currentKubernetesVersion),
    version = tostring(properties.kubernetesVersion),
    pools = properties.agentPoolProfiles,
    karpenter = coalesce(karpenter, 0),
    tier = tostring(sku.tier),
    channel = tostring(properties.autoUpgradeProfile.upgradeChannel)
| order by name asc`

// AKSClusters returns the AKS clusters of every subscription.
func AKSClusters(ctx context.Context, graph ResourceGraph) ([]Cluster, error) {
	return query[Cluster](ctx, graph, clustersQuery)
}
