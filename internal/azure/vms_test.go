package azure

import (
	"context"
	"errors"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resourcegraph/armresourcegraph"
)

// joins matches a join or a union; Resource Graph allows 3 of them in one query.
var joins = regexp.MustCompile(`\|\s*(join|union)\b`)

func TestVirtualMachines(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{
		page(nil, map[string]any{
			"id": "/subscriptions/s1/vm1", "name": "vm1", "type": "microsoft.compute/virtualmachines", "location": "northeurope",
			"resourceGroup": "rg-a", "subscriptionId": "s1", "subscription": "Dev",
			"power": "PowerState/running", "provisioning": "Succeeded", "size": "Standard_D2s_v5", "os": "Linux",
			"privateIp": "10.0.0.4", "publicIp": "20.1.2.3",
		}),
	}}

	got, err := VirtualMachines(context.Background(), graph)
	if err != nil {
		t.Fatal(err)
	}

	want := []VM{{
		Resource: Resource{
			ID: "/subscriptions/s1/vm1", Name: "vm1", Type: "microsoft.compute/virtualmachines", Location: "northeurope",
			ResourceGroup: "rg-a", SubscriptionID: "s1", Subscription: "Dev",
		},
		Power: "PowerState/running", Provisioning: "Succeeded", Size: "Standard_D2s_v5", OS: "Linux",
		PrivateIP: "10.0.0.4", PublicIP: "20.1.2.3",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("VirtualMachines() = %+v, want %+v", got, want)
	}
	if got := scope(graph.requests[0]); len(got) != 0 {
		t.Errorf("scope = %v, want empty", got)
	}
}

func TestVirtualMachinesQuery(t *testing.T) {
	graph := &fakeGraph{pages: []armresourcegraph.ClientResourcesResponse{page(nil)}}
	if _, err := VirtualMachines(context.Background(), graph); err != nil {
		t.Fatal(err)
	}

	q := *graph.requests[0].Query
	for _, want := range []string{
		"where type =~ 'microsoft.compute/virtualmachines'",
		"networkProfile.networkInterfaces[0].id",
		"where type =~ 'microsoft.network/networkinterfaces'",
		"ipConfigurations[0].properties.privateIPAddress",
		"ipConfigurations[0].properties.publicIPAddress.id",
		"where type =~ 'microsoft.network/publicipaddresses'",
		"publicIp = tostring(properties.ipAddress)",
		"| join kind=leftouter (\n    resources | where type =~ 'microsoft.network/networkinterfaces'",
		"| join kind=leftouter (\n    resources | where type =~ 'microsoft.network/publicipaddresses'",
		"on nicId",
		"on pipId",
		wantJoin,
		"power = tostring(properties.extended.instanceView.powerState.code)",
		"provisioning = tostring(properties.provisioningState)",
		"size = tostring(properties.hardwareProfile.vmSize)",
		"os = tostring(properties.storageProfile.osDisk.osType)",
		"    privateIp, publicIp\n| order by name asc",
		"| project id, name, type, location, resourceGroup, subscriptionId, subscription,",
		"| order by name asc",
	} {
		if !strings.Contains(q, want) {
			t.Errorf("query lacks %q:\n%s", want, q)
		}
	}
	if n := len(joins.FindAllString(q, -1)); n > 3 {
		t.Errorf("query joins %d times to resources, at most 3 are allowed", n)
	}
	if strings.Contains(q, " kind =") || strings.Contains(q, "project kind") {
		t.Errorf("query names a column kind:\n%s", q)
	}
}

func TestVirtualMachinesGraphError(t *testing.T) {
	boom := errors.New("boom")
	graph := &fakeGraph{err: boom}

	if _, err := VirtualMachines(context.Background(), graph); !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
}
