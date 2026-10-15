package views

import (
	"bytes"
	"encoding/json"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	jsonKey     = accentStyle
	jsonString  = goodStyle
	jsonNumber  = warnStyle
	jsonBoolean = lipgloss.NewStyle().Foreground(boolean)
	jsonNull    = mutedStyle
)

// jsonIndent is the indent of one level of JSON.
const jsonIndent = "  "

// encodeJSON encodes v, keeping &, < and > as they are; a non-empty indent
// spreads the result over lines.
func encodeJSON(v any, indent string) (string, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", indent)
	err := enc.Encode(v)
	return strings.TrimSuffix(b.String(), "\n"), err
}

// indentJSON encodes v with two-space indents.
func indentJSON(v any) (string, error) { return encodeJSON(v, jsonIndent) }

// jsonLines prints v the way indentJSON does, one colored line per element.
func jsonLines(v any) []string {
	var b strings.Builder
	printJSON(&b, v, "")
	return strings.Split(b.String(), "\n")
}

func printJSON(b *strings.Builder, v any, indent string) {
	switch v := v.(type) {
	case nil:
		b.WriteString(jsonNull.Render("null"))
	case bool:
		b.WriteString(jsonBoolean.Render(choose(v, "true", "false")))
	case string:
		b.WriteString(jsonString.Render(jsonText(v)))
	case float64:
		b.WriteString(jsonNumber.Render(jsonText(v)))
	case json.Number:
		b.WriteString(jsonNumber.Render(v.String()))
	case map[string]any:
		printMap(b, v, indent)
	case []any:
		printList(b, v, indent)
	default:
		printJSON(b, normalize(v), indent)
	}
}

// jsonText is a scalar as JSON text.
func jsonText(v any) string {
	out, _ := encodeJSON(v, "")
	return out
}

// normalize turns a value of another type into what decoding its JSON gives.
func normalize(v any) any {
	raw, err := encodeJSON(v, "")
	if err != nil {
		return nil
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var out any
	if dec.Decode(&out) != nil {
		return nil
	}
	return out
}

func printMap(b *strings.Builder, m map[string]any, indent string) {
	if m == nil {
		printJSON(b, nil, indent)
		return
	}
	keys := mapKeys(m)
	printItems(b, "{", "}", len(keys), indent, func(i int, inner string) {
		b.WriteString(jsonKey.Render(jsonText(keys[i])) + ": ")
		printJSON(b, m[keys[i]], inner)
	})
}

func printList(b *strings.Builder, l []any, indent string) {
	if l == nil {
		printJSON(b, nil, indent)
		return
	}
	printItems(b, "[", "]", len(l), indent, func(i int, inner string) { printJSON(b, l[i], inner) })
}

// printItems writes n items between the brackets, one per line, or the empty
// brackets when there are none.
func printItems(b *strings.Builder, open, closing string, n int, indent string, item func(i int, inner string)) {
	if n == 0 {
		b.WriteString(open + closing)
		return
	}
	inner := indent + jsonIndent
	b.WriteString(open + "\n")
	for i := range n {
		b.WriteString(inner)
		item(i, inner)
		if i < n-1 {
			b.WriteString(",")
		}
		b.WriteString("\n")
	}
	b.WriteString(indent + closing)
}
