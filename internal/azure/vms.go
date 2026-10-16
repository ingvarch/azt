package azure

import "context"

// VM is a virtual machine as the VM list query returns it.
type VM struct {
	Resource
	Power        string `json:"power"` // "PowerState/running"; empty without an instance view
	Provisioning string `json:"provisioning"`
	Size         string `json:"size"`
	OS           string `json:"os"`
	PrivateIP    string `json:"privateIp"` // of the first IP configuration of the first interface
	PublicIP     string `json:"publicIp"`
}

const vmsQuery = `resources
| where type =~ 'microsoft.compute/virtualmachines'
| extend nicId = tolower(tostring(properties.networkProfile.networkInterfaces[0].id))
| join kind=leftouter (
    resources | where type =~ 'microsoft.network/networkinterfaces'
    | project nicId = tolower(id), privateIp = tostring(properties.ipConfigurations[0].properties.privateIPAddress),
        pipId = tolower(tostring(properties.ipConfigurations[0].properties.publicIPAddress.id))
  ) on nicId
| join kind=leftouter (
    resources | where type =~ 'microsoft.network/publicipaddresses'
    | project pipId = tolower(id), publicIp = tostring(properties.ipAddress)
  ) on pipId
` + withSubscriptionName + `
| project id, name, type, location, resourceGroup, subscriptionId, subscription,
    power = tostring(properties.extended.instanceView.powerState.code),
    provisioning = tostring(properties.provisioningState),
    size = tostring(properties.hardwareProfile.vmSize),
    os = tostring(properties.storageProfile.osDisk.osType),
    privateIp, publicIp
| order by name asc`

// VirtualMachines returns the virtual machines of every subscription.
func VirtualMachines(ctx context.Context, graph ResourceGraph) ([]VM, error) {
	return query[VM](ctx, graph, vmsQuery)
}
