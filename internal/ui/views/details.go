package views

import (
	"bytes"
	"encoding/json"
	"strings"

	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/ingvarch/azt/internal/azure"
	"github.com/ingvarch/azt/internal/ui/data"
)

type details struct {
	loader
	title         string
	spec          Spec
	viewport      viewport.Model
	width, height int
}

// NewDetails shows one resource as indented JSON.
func NewDetails(subscriptionID string, r azure.Resource) View {
	return details{
		loader:   newLoader(data.Key{Kind: data.Details, Subscription: subscriptionID, ID: r.ID}, "details"),
		title:    r.Name,
		spec:     Spec{Kind: "details", Subscription: subscriptionID, ResourceID: r.ID, ResourceName: r.Name},
		viewport: viewport.New(),
	}
}

func (d details) Title() string { return d.title }

func (d details) Init() tea.Cmd { return d.init() }

func (d details) Update(msg tea.Msg) (View, tea.Cmd) {
	switch msg := msg.(type) {
	case data.ResultMsg:
		res, ok := d.result(msg)
		if !ok {
			return d, nil
		}
		if d.err == nil {
			var text string
			text, d.err = indentJSON(res)
			d.viewport.SetContent(text)
		}
		return d.fit(), nil
	case spinner.TickMsg:
		cmd := d.tick(msg)
		return d, cmd
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			cmd := d.refresh()
			return d.fit(), cmd
		}
	}
	var cmd tea.Cmd
	d.viewport, cmd = d.viewport.Update(msg)
	return d, cmd
}

func (d details) Resize(width, height int) View {
	d.width, d.height = width, height
	return d.fit()
}

// fit sizes the viewport, leaving a line for the refresh indicator.
func (d details) fit() details {
	d.viewport.SetWidth(d.width)
	d.viewport.SetHeight(d.height - lines(d.refreshing))
	return d
}

func (d details) Render() string {
	if d.width <= 0 || d.height <= 0 {
		return ""
	}
	if s, ok := d.status(d.width, d.height); ok {
		return s
	}
	if d.refreshing {
		return clip(lipgloss.JoinVertical(lipgloss.Left, faintStyle.Render(refreshingText), d.viewport.View()), d.width, d.height)
	}
	return d.viewport.View()
}

func (d details) Spec() Spec { return d.spec }

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
