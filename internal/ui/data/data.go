// Package data fetches Azure data for the views and caches it.
package data

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/azure"
)

type Kind int

const (
	Subscriptions Kind = iota
	ResourceGroups
	Resources
	Details
)

// Key identifies one piece of data; unused fields stay empty.
type Key struct {
	Kind         Kind
	Subscription string
	Group, ID    string
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

// NewFetcher reads each kind of data through graph.
func NewFetcher(ctx context.Context, graph azure.ResourceGraph) Fetcher {
	return func(k Key) (any, error) {
		switch k.Kind {
		case Subscriptions:
			return azure.Subscriptions(ctx, graph)
		case ResourceGroups:
			return azure.ResourceGroups(ctx, graph, k.Subscription)
		case Resources:
			return azure.Resources(ctx, graph, k.Subscription, k.Group)
		case Details:
			return azure.ResourceDetails(ctx, graph, k.Subscription, k.ID)
		}
		return nil, fmt.Errorf("unknown data kind %d", k.Kind)
	}
}
