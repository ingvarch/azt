package views

import (
	"slices"
	"sort"
	"strconv"
	"strings"
)

// dash stands for a value the row does not have.
const dash = "—"

// dig follows path through nested maps; it returns nil when a step is missing.
func dig(v any, path ...string) any {
	for _, k := range path {
		m, ok := v.(map[string]any)
		if !ok {
			return nil
		}
		v = m[k]
	}
	return v
}

// text is v as a string when it is a string or a number, else "".
func text(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

func str(v any, path ...string) string { return text(dig(v, path...)) }

// field is str, or dash when empty.
func field(v any, path ...string) string { return orDash(str(v, path...)) }

func orDash(s string) string {
	if s == "" {
		return dash
	}
	return s
}

// flag reads a boolean; ok is false when it is missing.
func flag(v any, path ...string) (on, ok bool) {
	on, ok = dig(v, path...).(bool)
	return on, ok
}

// flagText shows a boolean as on or off, dash when missing.
func flagText(on, off string, v any, path ...string) string {
	b, ok := flag(v, path...)
	switch {
	case !ok:
		return dash
	case b:
		return on
	}
	return off
}

func onOff(v any, path ...string) string { return flagText("on", "off", v, path...) }
func choose(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

// enabledState is "on" or "off" for a bool and the strings "enabled" and
// "disabled" in any case; any other text as it is, dash when there is none.
func enabledState(v any) string {
	if on, ok := v.(bool); ok {
		return choose(on, "on", "off")
	}
	switch s := text(v); strings.ToLower(s) {
	case "enabled":
		return "on"
	case "disabled":
		return "off"
	default:
		return orDash(s)
	}
}

// items returns the non-empty strings and numbers of a list.
func items(v any, path ...string) []string {
	var out []string
	for _, x := range elems(dig(v, path...)) {
		if s := text(x); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func elems(v any) []any {
	l, _ := v.([]any)
	return l
}

// joined is the items of a list separated by ", ", or dash.
func joined(v any, path ...string) string { return orDash(strings.Join(items(v, path...), ", ")) }

// mapKeys returns the keys of a map in order.
func mapKeys(v any) []string {
	m, _ := v.(map[string]any)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// first returns the first non-empty string.
func first(parts ...string) string {
	i := slices.IndexFunc(parts, func(s string) bool { return s != "" })
	if i < 0 {
		return ""
	}
	return parts[i]
}

// join joins the non-empty parts.
func join(sep string, parts ...string) string {
	return strings.Join(slices.DeleteFunc(slices.Clone(parts), func(s string) bool { return s == "" }), sep)
}

// lastSegment is the part of a resource ID after the last slash.
func lastSegment(s string) string { return s[strings.LastIndex(s, "/")+1:] }

// notSucceeded returns a provisioning state unless it is Succeeded.
func notSucceeded(state string) string {
	if strings.EqualFold(state, "Succeeded") {
		return ""
	}
	return state
}

// containsFold reports whether list holds s, ignoring case.
func containsFold(list []string, s string) bool {
	return slices.ContainsFunc(list, func(x string) bool { return strings.EqualFold(x, s) })
}
