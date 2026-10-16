package views

import (
	"slices"
	"strings"
	"testing"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

var vmRows = []azure.VM{
	{
		Resource:     azure.Resource{ID: "/r/web", Name: "vm-web", Type: vmType, Location: "westeurope", ResourceGroup: "rg-a", SubscriptionID: "id-1", Subscription: "Dev"},
		Power:        "PowerState/running",
		Provisioning: "Succeeded",
		Size:         "Standard_D2s_v5", OS: "Linux", PrivateIP: "10.0.0.4", PublicIP: "20.1.2.3",
	},
	{
		Resource:     azure.Resource{ID: "/r/batch", Name: "vm-batch", Type: vmType, Location: "northeurope", ResourceGroup: "rg-b", SubscriptionID: "id-2", Subscription: "Prod"},
		Power:        "PowerState/deallocated",
		Provisioning: "Succeeded",
		Size:         "Standard_B2s", OS: "Windows", PrivateIP: "10.1.0.5",
	},
	{
		Resource:     azure.Resource{ID: "/r/bad", Name: "vm-bad", Type: vmType, Location: "northeurope", ResourceGroup: "rg-b", SubscriptionID: "id-2", Subscription: "Prod"},
		Power:        "PowerState/stopped",
		Provisioning: "Failed",
		Size:         "Standard_B1s", OS: "Linux",
	},
}

func TestVirtualMachinesKeyTitleAndSpec(t *testing.T) {
	v := NewVirtualMachines()
	if got, want := fetchKey(t, v), (data.Key{Kind: data.VirtualMachines}); got != want {
		t.Errorf("key = %+v, want %+v", got, want)
	}
	if got, want := v.Title(), "Virtual machines"; got != want {
		t.Errorf("Title = %q, want %q", got, want)
	}
	if got, want := v.Spec(), (Spec{Kind: "resourcesByType", Type: vmType}); got != want {
		t.Errorf("Spec = %+v, want %+v", got, want)
	}
}

func TestVirtualMachinesColumns(t *testing.T) {
	header := plain(firstLine(deliver(t, NewVirtualMachines().Resize(200, 10), vmRows, nil)))
	var got []string
	for _, c := range []string{"Name", "Status", "Size", "OS", "Private IP", "Public IP", "Resource group", "Subscription", "Location"} {
		if !strings.Contains(header, c) {
			t.Fatalf("header %q lacks %q", header, c)
		}
		got = append(got, c)
	}
	if !slices.IsSortedFunc(got, func(a, b string) int { return strings.Index(header, a) - strings.Index(header, b) }) {
		t.Errorf("header %q has the columns out of order", header)
	}
}

func TestVMRow(t *testing.T) {
	want := [][]string{
		{"vm-web", "Running", "Standard_D2s_v5", "Linux", "10.0.0.4", "20.1.2.3", "rg-a", "Dev", "westeurope"},
		{"vm-batch", "Deallocated", "Standard_B2s", "Windows", "10.1.0.5", dash, "rg-b", "Prod", "northeurope"},
		{"vm-bad", "Stopped · Failed", "Standard_B1s", "Linux", dash, dash, "rg-b", "Prod", "northeurope"},
	}
	for i, vm := range vmRows {
		if got := vmRow(vm); !slices.Equal(got, want[i]) {
			t.Errorf("vmRow(%s) = %q, want %q", vm.Name, got, want[i])
		}
	}
}

func TestVMRowDashesMissingSizeAndOS(t *testing.T) {
	got := vmRow(azure.VM{Resource: azure.Resource{Name: "vm-new"}, Provisioning: "Creating"})
	if got[2] != dash || got[3] != dash {
		t.Errorf("size, os = %q, %q, want dashes", got[2], got[3])
	}
}

func TestVMRowWithoutInstanceView(t *testing.T) {
	if got := vmRow(azure.VM{Provisioning: "Succeeded"})[1]; got != dash {
		t.Errorf("status = %q, want a dash", got)
	}
}

func TestVirtualMachinesDrawStatusAsState(t *testing.T) {
	v := deliver(t, NewVirtualMachines().Resize(200, 10), vmRows, nil)
	line := styledLine(t, v, "vm-batch")
	wantFg(t, line, "● Deallocated", fgBad)
}

func TestVirtualMachinesFilterByStatus(t *testing.T) {
	v := filtered(deliver(t, NewVirtualMachines().Resize(200, 10), vmRows, nil), "deallocated")
	got := v.Render()
	if !strings.Contains(got, "vm-batch") || strings.Contains(got, "vm-web") || strings.Contains(got, "vm-bad") {
		t.Errorf("render:\n%s", got)
	}
}

func TestVirtualMachinesCount(t *testing.T) {
	v := NewVirtualMachines()
	if _, ok := v.(Counter).Count(); ok {
		t.Error("Count() ok while loading")
	}
	if n, ok := deliver(t, v, vmRows, nil).(Counter).Count(); !ok || n != len(vmRows) {
		t.Errorf("Count() = %d, %v, want %d, true", n, ok, len(vmRows))
	}
}

func TestVirtualMachinesOpen(t *testing.T) {
	v := deliver(t, NewVirtualMachines().Resize(200, 10), vmRows, nil)
	v, _ = press(v, keyDown)
	next, ok := v.Open()
	if !ok {
		t.Fatal("Open is false")
	}
	if got := next.Title(); got != "vm-batch" {
		t.Errorf("title = %q, want vm-batch", got)
	}
	if got, want := fetchKey(t, next), (data.Key{Kind: data.Details, Subscription: "id-2", ID: "/r/batch"}); got != want {
		t.Errorf("key = %+v, want %+v", got, want)
	}
}
