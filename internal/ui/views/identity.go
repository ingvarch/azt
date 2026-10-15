package views

var miTabs = []tabDef{
	{overviewTab, boxesTab(miEssentials)},
	{"JSON", jsonTab},
}

var miEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Client ID", path: []string{"properties", "clientId"}},
	{label: "Principal ID", path: []string{"properties", "principalId"}},
	{label: "Tenant ID", path: []string{"properties", "tenantId"}},
	{label: "Isolation scope", path: []string{"properties", "isolationScope"}},
	resourceGroupField,
	locationField,
	subscriptionField,
}}
