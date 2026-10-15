package views

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

const pgType = "microsoft.dbforpostgresql/flexibleservers"

// pgRow is a server row from JSON; its type is the server type.
func pgRow(t *testing.T, body string) map[string]any {
	t.Helper()
	var row map[string]any
	if err := json.Unmarshal([]byte(body), &row); err != nil {
		t.Fatal(err)
	}
	row["type"] = pgType
	return row
}

func TestPostgresOverview(t *testing.T) {
	// Inner width 104: two columns of 50 and a gap of 4.
	want := []string{
		"Essentials",
		twoCol("Status       ● Ready", "Zone            2", 50),
		twoCol("Version      17.6", "Resource group  rg-orders", 50),
		twoCol("SKU          Standard_D4ds_v5 (GeneralPurpose)", "Location        northeurope", 50),
		twoCol("Server name  pg-orders.postgres.database.azure.com", "Subscription    Prod", 50),
		twoCol("Admin        pgadmin", "", 50),
		"",
		"Storage and backup",
		"Size                  128 GB",
		"Type                  Premium_LRS",
		"Tier                  P10",
		"IOPS                  500",
		"Auto-grow             on",
		"Backup retention      14 days",
		"Geo-redundant backup  on",
		"Earliest restore      2025-08-12 23:45 UTC",
		"",
		"High availability",
		"Mode              ZoneRedundant",
		"State             Healthy",
		"Standby zone      3",
		"Replication role  Primary",
		"Replica capacity  7",
		"",
		"Network and auth",
		"Public network access  on",
		"Delegated subnet       vnet-hub/snet-pg",
		"Private DNS zone       pg-orders.private.postgres.database.azure.com",
		"Entra ID auth          on",
		"Password auth          off",
		"Maintenance window     Friday 04:30 UTC",
		"Data encryption        customer-managed key",
		"",
		"Attention",
		"▲ public network access is enabled",
		"",
		"Tags",
		"team = orders",
		"",
	}
	wantLines(t, bodyOf(t, "pg", "", 110), want)
}

// pgProbed is a server as Azure returns it: ready, private, no high availability.
const pgProbed = `{
	"resourceGroup": "rg-db", "location": "swedencentral", "subscription": "Dev",
	"sku": {"name": "Standard_B1ms", "tier": "Burstable"}, "identity": null, "zones": null, "tags": {"env": "test"},
	"properties": {
		"state": "Ready", "version": "16", "minorVersion": "15", "administratorLogin": "pgroot",
		"fullyQualifiedDomainName": "pg-dev.postgres.database.azure.com", "availabilityZone": "1",
		"authConfig": {"activeDirectoryAuth": "Disabled", "passwordAuth": "Enabled"},
		"backup": {"backupRetentionDays": 7, "geoRedundantBackup": "Enabled", "earliestRestoreDate": "2026-10-08T06:12:44.5Z"},
		"dataEncryption": {"type": "SystemManaged"},
		"highAvailability": {"mode": "Disabled", "state": "NotEnabled"},
		"maintenanceWindow": {"customWindow": "Disabled", "dayOfWeek": 0, "startHour": 0, "startMinute": 0},
		"network": {
			"delegatedSubnetResourceId": "/subscriptions/s1/resourceGroups/rg-net/providers/Microsoft.Network/virtualNetworks/vnet-dev/subnets/snet-db",
			"privateDnsZoneArmResourceId": "/subscriptions/s1/resourceGroups/rg-net/providers/Microsoft.Network/privateDnsZones/pg-dev.private.postgres.database.azure.com",
			"publicNetworkAccess": "Disabled"
		},
		"replica": {"role": "Primary", "capacity": 5}, "replicationRole": "Primary", "replicaCapacity": 5,
		"storage": {"autoGrow": "Disabled", "iops": 120, "storageSizeGB": 32, "tier": "P4", "type": "Premium_LRS"}
	}
}`

func TestPostgresOverviewOneColumn(t *testing.T) {
	want := []string{
		"Essentials",
		"Status          ● Ready",
		"Version         16.15",
		"SKU             Standard_B1ms (Burstable)",
		"Server name     pg-dev.postgres.database.azure.com",
		"Admin           pgroot",
		"Zone            1",
		"Resource group  rg-db",
		"Location        swedencentral",
		"Subscription    Dev",
		"",
		"Storage and backup",
		"Size                  32 GB",
		"Type                  Premium_LRS",
		"Tier                  P4",
		"IOPS                  120",
		"Auto-grow             off",
		"Backup retention      7 days",
		"Geo-redundant backup  on",
		"Earliest restore      2026-10-08 06:12 UTC",
		"",
		"High availability",
		"Mode              off",
		"State             NotEnabled",
		"Standby zone      —",
		"Replication role  Primary",
		"Replica capacity  5",
		"",
		"Network and auth",
		"Public network access  off",
		"Delegated subnet       vnet-dev/snet-db",
		"Private DNS zone       pg-dev.private.postgres.database.azure.com",
		"Entra ID auth          off",
		"Password auth          on",
		"Maintenance window     System managed",
		"Data encryption        Microsoft-managed key",
		"",
		"Tags",
		"env = test",
		"",
	}
	wantLines(t, overviewBody(t, pgRow(t, pgProbed), 80), want)
}

func TestPostgresMissingFields(t *testing.T) {
	want := []string{
		"Essentials",
		"Status          —",
		"Version         —",
		"SKU             —",
		"Server name     —",
		"Admin           —",
		"Zone            —",
		"Resource group  —",
		"Location        —",
		"Subscription    —",
		"",
		"Storage and backup",
		"Size                  —",
		"Type                  —",
		"Tier                  —",
		"IOPS                  —",
		"Auto-grow             —",
		"Backup retention      —",
		"Geo-redundant backup  —",
		"Earliest restore      —",
		"",
		"High availability",
		"Mode              —",
		"State             —",
		"Standby zone      —",
		"Replication role  —",
		"Replica capacity  —",
		"",
		"Network and auth",
		"Public network access  —",
		"Delegated subnet       —",
		"Private DNS zone       —",
		"Entra ID auth          —",
		"Password auth          —",
		"Maintenance window     System managed",
		"Data encryption        —",
		"",
		"Tags",
		"none",
		"",
	}
	wantLines(t, overviewBody(t, map[string]any{"type": pgType}, 80), want)
}

func TestPostgresValues(t *testing.T) {
	for _, c := range []struct{ body, label, want string }{
		{`{"properties":{"version":"16","minorVersion":"15"}}`, "Version", "16.15"},
		{`{"properties":{"version":"16"}}`, "Version", "16"},
		{`{"properties":{"minorVersion":"15"}}`, "Version", "—"},
		{`{"sku":{"name":"Standard_B1ms","tier":"Burstable"}}`, "SKU", "Standard_B1ms (Burstable)"},
		{`{"sku":{"name":"Standard_B1ms"}}`, "SKU", "Standard_B1ms"},
		{`{"sku":{"tier":"Burstable"}}`, "SKU", "—"},
		{`{"properties":{"backup":{"earliestRestoreDate":"soon"}}}`, "Earliest restore", "soon"},
		{`{"properties":{"highAvailability":{"mode":"SameZone","state":"CreatingStandby","standbyAvailabilityZone":"2"}}}`, "Mode", "SameZone"},
		{`{"properties":{"highAvailability":{"mode":"SameZone","state":"CreatingStandby","standbyAvailabilityZone":"2"}}}`, "State", "CreatingStandby"},
		{`{"properties":{"highAvailability":{"mode":"SameZone","state":"CreatingStandby","standbyAvailabilityZone":"2"}}}`, "Standby zone", "2"},
		{`{"properties":{"replica":{"role":"Primary","capacity":5},"replicationRole":"AsyncReplica","replicaCapacity":9}}`, "Replication role", "Primary"},
		{`{"properties":{"replica":{"role":"Primary","capacity":5},"replicationRole":"AsyncReplica","replicaCapacity":9}}`, "Replica capacity", "5"},
		{`{"properties":{"replicationRole":"AsyncReplica","replicaCapacity":9}}`, "Replication role", "AsyncReplica"},
		{`{"properties":{"replicationRole":"AsyncReplica","replicaCapacity":9}}`, "Replica capacity", "9"},
		{`{"properties":{"replica":{"role":"Primary"},"replicaCapacity":9}}`, "Replica capacity", "9"},
		{`{"properties":{"network":{"delegatedSubnetResourceId":""}}}`, "Delegated subnet", "—"},
		{`{"properties":{"network":{"delegatedSubnetResourceId":"/x/subnets/s1"}}}`, "Delegated subnet", "x/s1"},
		{`{"properties":{"network":{"privateDnsZoneArmResourceId":""}}}`, "Private DNS zone", "—"},
		{`{"properties":{"maintenanceWindow":{"customWindow":"Enabled","dayOfWeek":0,"startHour":0,"startMinute":0}}}`, "Maintenance window", "Sunday 00:00 UTC"},
		{`{"properties":{"maintenanceWindow":{"customWindow":"Enabled","dayOfWeek":6,"startHour":23,"startMinute":5}}}`, "Maintenance window", "Saturday 23:05 UTC"},
		{`{"properties":{"maintenanceWindow":{"customWindow":"enabled","dayOfWeek":1,"startHour":9,"startMinute":0}}}`, "Maintenance window", "Monday 09:00 UTC"},
		{`{"properties":{"maintenanceWindow":{"customWindow":"Disabled","dayOfWeek":3,"startHour":4,"startMinute":30}}}`, "Maintenance window", "System managed"},
		{`{"properties":{"maintenanceWindow":{}}}`, "Maintenance window", "System managed"},
		{`{"properties":{"dataEncryption":{"type":"SystemManaged"}}}`, "Data encryption", "Microsoft-managed key"},
		{`{"properties":{"dataEncryption":{"type":"systemmanaged"}}}`, "Data encryption", "Microsoft-managed key"},
		{`{"properties":{"dataEncryption":{"type":"AzureKeyVault"}}}`, "Data encryption", "customer-managed key"},
		{`{"properties":{"dataEncryption":{"type":"Other"}}}`, "Data encryption", "Other"},
		{`{"properties":{"highAvailability":{"mode":"disabled"}}}`, "Mode", "off"},
	} {
		got := valueOf(overviewBody(t, pgRow(t, c.body), 80), c.label)
		if got != c.want {
			t.Errorf("%s: %s = %q, want %q", c.body, c.label, got, c.want)
		}
	}
}

func TestPostgresFlagsReadTheirOwnProperty(t *testing.T) {
	flags := []struct{ label, path, on string }{
		{"Auto-grow", `"storage":{"autoGrow"`, `"Enabled"}`},
		{"Geo-redundant backup", `"backup":{"geoRedundantBackup"`, `"Enabled"}`},
		{"Public network access", `"network":{"publicNetworkAccess"`, `"Enabled"}`},
		{"Entra ID auth", `"authConfig":{"activeDirectoryAuth"`, `"Enabled"}`},
		{"Password auth", `"authConfig":{"passwordAuth"`, `"Enabled"}`},
	}
	for _, f := range flags {
		body := `{"properties":{` + f.path + ":" + f.on + "}}"
		lines := overviewBody(t, pgRow(t, body), 80)
		for _, other := range flags {
			want := "—"
			if other.label == f.label {
				want = "on"
			}
			if got := valueOf(lines, other.label); got != want {
				t.Errorf("only %s on: %s = %q, want %q", f.label, other.label, got, want)
			}
		}
	}
}

func TestPostgresNumbersReadTheirOwnProperty(t *testing.T) {
	lines := overviewBody(t, pgRow(t, `{"properties":{
		"storage":{"iops":300,"storageSizeGB":64},
		"backup":{"backupRetentionDays":21},
		"replica":{"capacity":4}}}`), 80)
	for label, want := range map[string]string{
		"IOPS": "300", "Size": "64 GB", "Backup retention": "21 days", "Replica capacity": "4",
	} {
		if got := valueOf(lines, label); got != want {
			t.Errorf("%s = %q, want %q", label, got, want)
		}
	}
}

func TestPostgresColors(t *testing.T) {
	v := openedAt(t, "pg", "", 110, 80)
	wantFg(t, styledLine(t, v, "Status"), "Ready", fgGood)
	wantFg(t, styledLine(t, v, "Auto-grow"), "on", fgGood)
	wantFg(t, styledLine(t, v, "Password auth"), "off", fgMuted)
	wantFg(t, styledLine(t, v, "Entra ID auth"), "on", fgGood)
	wantFg(t, styledLine(t, v, "Mode"), "ZoneRedundant", "")
	wantFg(t, styledLine(t, v, "Public network access"), "on", fgGood)

	off := deliver(t, openedAtRow(t, "").Resize(110, 80), pgRow(t, pgProbed), nil)
	wantFg(t, styledLine(t, off, "Mode"), "off", fgMuted)
}

func TestPostgresStateColors(t *testing.T) {
	for state, fg := range map[string]string{"Ready": fgGood, "Stopped": fgBad, "Dropping": fgBad, "Starting": fgWarn} {
		v := deliver(t, openedAtRow(t, "").Resize(110, 80), pgRow(t, `{"properties":{"state":"`+state+`"}}`), nil)
		wantFg(t, styledLine(t, v, "Status"), state, fg)
	}
}

func TestPostgresAttention(t *testing.T) {
	const (
		stopped  = "▲ Stopped: Azure starts it again after seven days"
		disabled = "✗ the server is Disabled"
		dropping = "✗ the server is Dropping"
		public   = "▲ public network access is enabled"
	)
	tests := []struct {
		name string
		body string
		want []string
	}{
		{"nothing to report", `{"properties":{"state":"Ready","network":{"publicNetworkAccess":"Disabled"}}}`, nil},
		{"nothing at all", `{}`, nil},
		{"stopped", `{"properties":{"state":"Stopped"}}`, []string{stopped}},
		{"stopped in another case", `{"properties":{"state":"stopped"}}`, []string{stopped}},
		{"disabled", `{"properties":{"state":"Disabled"}}`, []string{disabled}},
		{"dropping", `{"properties":{"state":"Dropping"}}`, []string{dropping}},
		{"updating", `{"properties":{"state":"Updating"}}`, nil},
		{"public access", `{"properties":{"network":{"publicNetworkAccess":"Enabled"}}}`, []string{public}},
		{"public access in another case", `{"properties":{"network":{"publicNetworkAccess":"enabled"}}}`, []string{public}},
		{"public access missing", `{"properties":{"network":{}}}`, nil},
		{"stopped and public", `{"properties":{"state":"Stopped","network":{"publicNetworkAccess":"Enabled"}}}`, []string{stopped, public}},
		{"disabled and public", `{"properties":{"state":"Disabled","network":{"publicNetworkAccess":"Enabled"}}}`, []string{disabled, public}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := alertLines(pgAttention(pgRow(t, tt.body)))
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("attention = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPostgresAttentionSeverity(t *testing.T) {
	severe := func(body string) []bool {
		var out []bool
		for _, a := range pgAttention(pgRow(t, body)) {
			out = append(out, a.severe)
		}
		return out
	}
	for body, want := range map[string][]bool{
		`{"properties":{"state":"Stopped"}}`:                           {false},
		`{"properties":{"state":"Disabled"}}`:                          {true},
		`{"properties":{"state":"Dropping"}}`:                          {true},
		`{"properties":{"network":{"publicNetworkAccess":"Enabled"}}}`: {false},
	} {
		if got := severe(body); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: severe = %v, want %v", body, got, want)
		}
	}
}

func TestPostgresAttentionBox(t *testing.T) {
	body := overviewBody(t, pgRow(t, `{"properties":{"state":"Dropping"}}`), 80)
	if !strings.Contains(strings.Join(body, "\n"), "Attention\n✗ the server is Dropping\n") {
		t.Errorf("no error in the Attention box:\n%s", strings.Join(body, "\n"))
	}
}

func TestPostgresTabs(t *testing.T) {
	if got := Tabs(pgType); !reflect.DeepEqual(got, []string{"Overview", "JSON"}) {
		t.Errorf("Tabs = %v", got)
	}
	if k := kindOf("Microsoft.DBforPostgreSQL/flexibleServers"); k.command != "pg" || k.attention == nil {
		t.Errorf("kindOf in another case = %+v, want pg with attention", k)
	}
}
