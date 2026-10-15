package views

import (
	"bytes"
	"encoding/json"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

type details struct {
	loader
	title         string
	viewport      viewport.Model
	width, height int
}

// NewDetails shows one resource as indented JSON.
func NewDetails(subscriptionID string, r azure.Resource) View {
	return details{
		loader:   newLoader(data.Key{Kind: data.Details, Subscription: subscriptionID, ID: r.ID}, "details"),
		title:    r.Name,
		viewport: viewport.New(),
	}
}

func (d details) Title() string { return d.title }

func (d details) Init() tea.Cmd { return d.init() }

func (d details) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case data.ResultMsg:
		if res, ok := d.result(msg); ok && d.err == nil {
			var text string
			text, d.err = indentJSON(res)
			d.viewport.SetContent(text)
		}
		return d, nil
	case spinner.TickMsg:
		return d, d.tick(msg)
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			return d, data.Refresh(d.key)
		}
	}
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

func (d details) Resize(width, height int) View {
	d.width, d.height = width, height
	d.viewport.SetWidth(width)
	d.viewport.SetHeight(height)
	return d
}

func (d details) Render() string {
	if d.width <= 0 || d.height <= 0 {
		return ""
	}
	if s, ok := d.status(d.width, d.height); ok {
		return s
	}
	return d.viewport.View()
}

func (details) Open() (View, bool) { return nil, false }

// indentJSON encodes v with two-space indents and keeps &, < and > as they are.
func indentJSON(v any) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	err := enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n"), err
}
