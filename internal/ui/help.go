package ui

import "strings"

const (
	helpMaxWidth = 76
	helpMargin   = 4 // cells left free around the box on each axis, in total
)

type helpEntry struct{ keys, text string }

var helpSections = []struct {
	title   string
	entries []helpEntry
}{
	{"Panes", []helpEntry{
		{"enter", "open the selected item"},
		{"esc", "back to the previous view"},
		{"j k / ↑ ↓", "move the cursor"},
		{"tab / shift+tab", "next / previous tab of a resource"},
		{"/", "filter the list (enter keeps, esc clears)"},
		{"r", "refresh"},
		{"?", "this help"},
		{"q", "quit"},
		{"ctrl+c", "quit from any mode"},
	}},
	{"Layout (ctrl+w)", []helpEntry{
		{"v / s", "split into left and right / top and bottom"},
		{"h j k l / arrows", "focus the neighbor pane"},
		{"q", "close the pane"},
		{"z", "zoom the pane"},
		{"=", "equalize the panes"},
		{"< > + -", "resize the pane"},
		{"t", "new tab"},
		{"n p 1-9", "next, previous, numbered tab"},
		{",", "rename the tab"},
		{"e", "show or hide the explorer"},
		{"?", "this help"},
		{"esc", "cancel"},
	}},
	{"Explorer", []helpEntry{
		{"j k / ↑ ↓", "move"},
		{"l h / → ←", "expand / collapse"},
		{"enter", "open in the focused pane"},
		{":", "command dialog"},
		{"?", "this help"},
		{"r", "refresh"},
		{"ctrl+w l", "back to the panes"},
	}},
	{"Commands", []helpEntry{
		{":", "command dialog (↑ ↓ choose, tab completes)"},
		{"subs  rg", "all subscriptions, all resource groups"},
		{"vm aks kv sql", "all resources of that type, in every subscription"},
		{"storage app vnet", "storage accounts, web apps, virtual networks"},
		{"mi acr", "managed identities, container registries"},
		{"nsg dns", "network security groups, public and private DNS zones"},
		{"type <type>", "all resources of a resource type"},
		{"help  q", "this help, quit"},
	}},
}

// helpLines is the text of the help box: sections with a blank line between.
func helpLines() []string {
	keyWidth := 0
	for _, s := range helpSections {
		for _, e := range s.entries {
			keyWidth = max(keyWidth, len([]rune(e.keys)))
		}
	}
	var out []string
	for i, s := range helpSections {
		if i > 0 {
			out = append(out, "")
		}
		out = append(out, s.title)
		for _, e := range s.entries {
			pad := strings.Repeat(" ", keyWidth-len([]rune(e.keys)))
			out = append(out, "  "+e.keys+pad+"  "+e.text)
		}
	}
	return out
}
