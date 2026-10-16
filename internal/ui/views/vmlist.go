package views

import (
	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

// NewVirtualMachines lists the virtual machines of every subscription.
func NewVirtualMachines() View {
	return newList(listConfig[azure.VM]{
		key:   data.Key{Kind: data.VirtualMachines},
		spec:  Spec{Kind: "resourcesByType", Type: vmType},
		title: KindName(vmType),
		noun:  "resources",
		empty: "No resources of this type.",
		columns: []column{
			{title: "Name"},
			{title: "Status", state: true},
			{title: "Size"},
			{title: "OS"},
			{title: "Private IP"},
			{title: "Public IP"},
			{title: "Resource group"},
			{title: "Subscription"},
			{title: "Location"},
		},
		row:  vmRow,
		open: func(vm azure.VM) View { return NewDetails(vm.SubscriptionID, vm.Resource) },
	})
}

func vmRow(vm azure.VM) []string {
	status := join(stateSeparator, powerWord(vm.Power), notSucceeded(vm.Provisioning))
	return []string{
		vm.Name, orDash(status), orDash(vm.Size), orDash(vm.OS), orDash(vm.PrivateIP), orDash(vm.PublicIP),
		vm.ResourceGroup, vm.Subscription, vm.Location,
	}
}
