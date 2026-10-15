package views

var foundryTabs = []tabDef{
	{overviewTab, boxesTab(foundryEssentials, foundryAccess, foundryProjects)},
	{"Endpoints", boxesTab(foundryEndpoints)},
	{"JSON", jsonTab},
}

var foundryProjectTabs = []tabDef{
	{overviewTab, boxesTab(foundryProjectEssentials, foundryEndpoints)},
	{"JSON", jsonTab},
}

var foundryEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Status", path: propPath("provisioningState"), format: fmtState},
	{label: "Kind", path: []string{"kind"}},
	{label: "SKU", path: []string{"sku", "name"}},
	{label: "Endpoint", path: propPath("endpoint")},
	{label: "Custom subdomain", path: propPath("customSubDomainName")},
	{label: "Created", path: propPath("dateCreated"), format: fmtDate},
	identityField,
	resourceGroupField,
	locationField,
	subscriptionField,
}}

var foundryAccess = boxDef{title: "Access", fields: []fieldDef{
	{label: "Local key auth", value: foundryLocalAuthText},
	publicAccessField,
	defaultActionField,
	privateEndpointsField,
	{label: "Project management", path: propPath("allowProjectManagement"), format: fmtOnOffOr(false)},
}}

var foundryProjects = boxDef{title: "Projects", table: &tableDef{
	list: foundryProjectList,
	cols: []colDef{
		{title: "Project", path: []string{"name"}},
		{title: "Default", path: []string{"default"}, format: fmtYesNo},
	},
}}

var foundryEndpoints = boxDef{title: "Endpoints", table: &tableDef{
	list: foundryEndpointList,
	cols: []colDef{
		{title: "Name", path: []string{"name"}},
		{title: "URL", path: []string{"url"}},
	},
}}

var foundryProjectEssentials = boxDef{title: "Essentials", split: true, fields: []fieldDef{
	{label: "Display name", path: propPath("displayName")},
	{label: "Description", path: propPath("description")},
	{label: "Default project", path: propPath("isDefault"), format: fmtYesNo},
	{label: "Status", path: propPath("provisioningState"), format: fmtState},
	{label: "Account", value: foundryAccountName},
	identityField,
	resourceGroupField,
	locationField,
	subscriptionField,
}}

// foundryLocalAuth reports key authentication; Azure has it on unless disableLocalAuth is true.
func foundryLocalAuth(row map[string]any) bool {
	disabled, _ := flag(row, propPath("disableLocalAuth")...)
	return !disabled
}

// foundryLocalAuthText is "on" in warn, or "off".
func foundryLocalAuthText(row map[string]any) string {
	if foundryLocalAuth(row) {
		return warnStyle.Render("on")
	}
	return switchText("off")
}

// foundryProjectList has one element per associated project, marking the default one.
func foundryProjectList(row map[string]any) []any {
	def := str(row, propPath("defaultProject")...)
	var out []any
	for _, name := range items(row, propPath("associatedProjects")...) {
		out = append(out, map[string]any{"name": name, "default": name == def})
	}
	return out
}

// foundryEndpointList has one element per endpoint, sorted by name.
func foundryEndpointList(row map[string]any) []any {
	endpoints := dig(row, propPath("endpoints")...)
	var out []any
	for _, name := range mapKeys(endpoints) {
		out = append(out, map[string]any{"name": name, "url": dig(endpoints, name)})
	}
	return out
}

// foundryAccountName is the account segment of a project's ID.
func foundryAccountName(row map[string]any) string {
	return orDash(segmentAfter(str(row, "id"), "accounts"))
}

// foundryAttention lists what needs a look in an account row.
func foundryAttention(row map[string]any) []alert {
	var out []alert
	if foundryLocalAuth(row) {
		out = append(out, alert{text: "local key auth is enabled"})
	}
	if openToAnyNetwork(row) {
		out = append(out, alert{text: "the account accepts connections from any network"})
	}
	return out
}
