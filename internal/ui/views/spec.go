package views

import (
	"fmt"

	"github.com/ingvarch/azt/internal/azure"
)

// Spec describes a view well enough to build it again; Build is its inverse.
type Spec struct {
	Kind             string `json:"kind"`
	Subscription     string `json:"subscription,omitempty"`
	SubscriptionName string `json:"subscriptionName,omitempty"`
	Group            string `json:"group,omitempty"`
	Type             string `json:"type,omitempty"`
	ResourceID       string `json:"resourceId,omitempty"`
	ResourceName     string `json:"resourceName,omitempty"`
	Filter           string `json:"filter,omitempty"`
	Tab              string `json:"tab,omitempty"`
}

// Build makes the view s describes and applies its filter.
func Build(s Spec) (View, error) {
	v, err := build(s)
	if err != nil {
		return nil, err
	}
	if f, ok := v.(Filterer); ok {
		v = f.SetFilter(s.Filter)
	}
	return v, nil
}

func build(s Spec) (View, error) {
	switch s.Kind {
	case "subscriptions":
		return NewSubscriptions(), nil
	case "resourceGroups":
		if err := require(s, "subscription", s.Subscription, "subscriptionName", s.SubscriptionName); err != nil {
			return nil, err
		}
		return NewResourceGroups(azure.Subscription{ID: s.Subscription, Name: s.SubscriptionName}), nil
	case "resources":
		if err := require(s, "subscription", s.Subscription, "group", s.Group); err != nil {
			return nil, err
		}
		return NewResources(s.Subscription, azure.ResourceGroup{Name: s.Group}), nil
	case "resourcesOfType":
		if err := require(s, "subscription", s.Subscription, "group", s.Group, "type", s.Type); err != nil {
			return nil, err
		}
		return NewResourcesOfType(s.Subscription, azure.ResourceGroup{Name: s.Group}, s.Type), nil
	case "details":
		if err := require(s, "subscription", s.Subscription, "resourceId", s.ResourceID, "resourceName", s.ResourceName); err != nil {
			return nil, err
		}
		return NewDetailsAt(s.Subscription, azure.Resource{ID: s.ResourceID, Name: s.ResourceName}, s.Tab), nil
	case "allResourceGroups":
		return NewAllResourceGroups(), nil
	case "resourcesByType":
		if err := require(s, "type", s.Type); err != nil {
			return nil, err
		}
		return NewResourcesByType(s.Type), nil
	}
	return nil, fmt.Errorf("unknown view kind %q", s.Kind)
}

// require takes pairs of a field name and its value and fails on the first empty value.
func require(s Spec, pairs ...string) error {
	for i := 0; i < len(pairs); i += 2 {
		if pairs[i+1] == "" {
			return fmt.Errorf("%s view needs %s", s.Kind, pairs[i])
		}
	}
	return nil
}
