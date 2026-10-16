// Package data fetches Azure data for the views and caches it.
package data

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/azure"
)

// Kind is the type of data a Key asks for.
type Kind int

// Subscriptions and the kinds up to AKSClusters each map to one Resource
// Graph query; AKSMonitoring reads Azure Monitor and diagnostic settings too.
const (
	Subscriptions Kind = iota
	ResourceGroups
	Resources
	Details
	AllResourceGroups
	ResourcesOfType
	VirtualMachines
	AKSClusters
	AKSMonitoring
)

// Key identifies one piece of data; unused fields stay empty.
type Key struct {
	Kind         Kind
	Subscription string
	Group, ID    string
	// Type is one resource type or several joined by ","; a type never holds a comma.
	Type string
	// Period is one of azure.Periods; only the monitoring kinds use it.
	Period string
}

// FetchMsg is sent by a view that needs data.
type FetchMsg struct {
	Key   Key
	Force bool
}

// ResultMsg carries the data for Key; the root model sends it to every view.
type ResultMsg struct {
	Key  Key
	Data any
	Err  error
}

// Fetcher loads the data for a Key; it may block on the network.
type Fetcher func(Key) (any, error)

// Fetch asks for the data of k, served from the cache when present.
func Fetch(k Key) tea.Cmd {
	return func() tea.Msg { return FetchMsg{Key: k} }
}

// Refresh asks for the data of k and skips the cache.
func Refresh(k Key) tea.Cmd {
	return func() tea.Msg { return FetchMsg{Key: k, Force: true} }
}

// Cache is not safe for concurrent use: only the root model's Update touches it.
type Cache struct {
	fetch    Fetcher
	data     map[Key]any
	inFlight map[Key]bool
}

// NewCache returns an empty cache that loads missing keys with f.
func NewCache(f Fetcher) *Cache {
	return &Cache{fetch: f, data: map[Key]any{}, inFlight: map[Key]bool{}}
}

// Request answers from the cache or starts a fetch; it returns nil while a
// fetch for the key is already running, even if the key has cached data.
func (c *Cache) Request(msg FetchMsg) tea.Cmd {
	k := msg.Key
	if c.inFlight[k] {
		return nil
	}
	if cached, ok := c.data[k]; ok && !msg.Force {
		return func() tea.Msg { return ResultMsg{Key: k, Data: cached} }
	}
	c.inFlight[k] = true
	return func() tea.Msg {
		d, err := c.fetch(k)
		return ResultMsg{Key: k, Data: d, Err: err}
	}
}

// Store records a fetch result; a failed fetch leaves nothing cached.
func (c *Cache) Store(msg ResultMsg) {
	delete(c.inFlight, msg.Key)
	if msg.Err != nil {
		delete(c.data, msg.Key)
		return
	}
	c.data[msg.Key] = msg.Data
}

// NewFetcher reads each kind of data through graph and arm.
func NewFetcher(ctx context.Context, graph azure.ResourceGraph, arm azure.ARM) Fetcher {
	return func(k Key) (any, error) {
		switch k.Kind {
		case Subscriptions:
			return azure.Subscriptions(ctx, graph)
		case ResourceGroups:
			return azure.ResourceGroups(ctx, graph, k.Subscription)
		case Resources:
			return azure.Resources(ctx, graph, k.Subscription, k.Group)
		case AllResourceGroups:
			return azure.AllResourceGroups(ctx, graph)
		case ResourcesOfType:
			return azure.ResourcesOfType(ctx, graph, strings.Split(k.Type, ","))
		case VirtualMachines:
			return azure.VirtualMachines(ctx, graph)
		case AKSClusters:
			return azure.AKSClusters(ctx, graph)
		case Details:
			return azure.ResourceDetails(ctx, graph, k.Subscription, k.ID)
		case AKSMonitoring:
			return azure.AKSMonitoring(ctx, graph, arm, k.Subscription, k.ID, k.Period, time.Now()), nil
		}
		return nil, fmt.Errorf("unknown data kind %d", k.Kind)
	}
}
