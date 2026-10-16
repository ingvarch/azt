package views

import (
	"testing"
)

func TestKindName(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"microsoft.compute/virtualmachines", "Virtual machines"},
		{"Microsoft.Compute/VirtualMachines", "Virtual machines"},
		{"MICROSOFT.CognitiveServices/accounts/projects", "AI Foundry projects"},
		{"microsoft.web/serverfarms", "web/serverfarms"},
		{"", ""},
	} {
		if got := KindName(c.in); got != c.want {
			t.Errorf("KindName(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEveryKindHasAName(t *testing.T) {
	for _, k := range kinds {
		if k.name == "" {
			t.Errorf("kind of %v has no name", k.types)
		}
	}
}

func TestResourcesByTypeTitles(t *testing.T) {
	for _, tt := range []struct {
		types []string
		want  string
	}{
		{[]string{"microsoft.compute/virtualmachines"}, "Virtual machines"},
		{[]string{dnsType, pdnsType}, "DNS zones"},
		{[]string{"microsoft.cognitiveservices/accounts", "microsoft.cognitiveservices/accounts/projects"}, "AI Foundry"},
		{[]string{"microsoft.web/serverfarms"}, "web/serverfarms"},
	} {
		if got := NewResourcesByType(tt.types...).Title(); got != tt.want {
			t.Errorf("NewResourcesByType(%v).Title() = %q, want %q", tt.types, got, tt.want)
		}
	}
}
