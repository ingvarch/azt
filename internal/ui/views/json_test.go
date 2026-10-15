package views

import (
	"encoding/json"
	"strings"
	"testing"
)

func plainJSON(v any) string { return strings.Join(plainAll(jsonLines(v)), "\n") }

func TestJSONLinesMatchIndentJSON(t *testing.T) {
	values := map[string]any{
		"empty map":        map[string]any{},
		"empty list":       []any{},
		"nil map":          map[string]any(nil),
		"nil list":         []any(nil),
		"nil":              nil,
		"string":           "a \"quoted\" line\n& <tag> é 日本   \x00",
		"bad utf8":         "a\xffb",
		"numbers":          []any{0.0, -1.5, 1e21, 1e-7, 123456789.125, 3, int64(-4), int64(1<<53 + 1), json.Number("12.50")},
		"booleans":         []any{true, false, nil},
		"nested":           map[string]any{"b": []any{map[string]any{}, []any{}, []any{1.0, "x"}}, "a": map[string]any{"z": nil, "y": true}},
		"keys to escape":   map[string]any{"a\"b": 1.0, "<k>": 2.0, "": 3.0},
		"typed containers": map[string]any{"s": []string{"x", "y"}, "m": map[string]int{"b": 2, "a": 1}},
	}
	for _, name := range []string{"aks", "aks_min", "aks_stopped", "webapp"} {
		values["fixture "+name] = fixture(t, name)
	}
	for name, v := range values {
		t.Run(name, func(t *testing.T) {
			want, err := indentJSON(v)
			if err != nil {
				t.Fatal(err)
			}
			if got := plainJSON(v); got != want {
				t.Errorf("got:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

func TestJSONLinesColors(t *testing.T) {
	lines := jsonLines(map[string]any{
		"name": "x", "n": 3.0, "ok": true, "no": false, "nothing": nil,
		"typed": int64(7), "list": []any{"a", 1.5}, "obj": map[string]any{"k": "v"}, "empty": map[string]any{},
	})
	line := func(sub string) string {
		t.Helper()
		for _, l := range lines {
			if strings.Contains(plain(l), sub) {
				return l
			}
		}
		t.Fatalf("no line with %q", sub)
		return ""
	}
	for _, c := range []struct{ line, sub, fg string }{
		{`"name"`, `"name"`, fgAccent},
		{`"name"`, `"x"`, fgGood},
		{`"name"`, `:`, ""},
		{`"name"`, `,`, ""},
		{`"n"`, `"n"`, fgAccent},
		{`"n"`, `3`, fgWarn},
		{`"typed"`, `7`, fgWarn},
		{`"ok"`, `true`, fgBoolean},
		{`"no"`, `false`, fgBoolean},
		{`"nothing"`, `null`, fgMuted},
		{`"list"`, `[`, ""},
		{`"a"`, `"a"`, fgGood},
		{`1.5`, `1.5`, fgWarn},
		{`"obj"`, `{`, ""},
		{`"k"`, `"k"`, fgAccent},
		{`"k"`, `"v"`, fgGood},
		{`"empty"`, `{}`, ""},
	} {
		if got := attrsOf(t, line(c.line), c.sub); got.fg != c.fg || got.bold {
			t.Errorf("%q on the line with %q = %+v, want foreground %q", c.sub, c.line, got, c.fg)
		}
	}
	if got := plain(lines[0]); got != "{" {
		t.Errorf("first line = %q", got)
	}
	if a := attrsOf(t, lines[0], "{"); a.fg != "" {
		t.Errorf("brace = %+v", a)
	}
}

func TestJSONTabIsColored(t *testing.T) {
	v := openedAt(t, "webapp", "JSON", 100, 200)
	wantFg(t, styledLine(t, v, `"name": "web-demo"`), `"name"`, fgAccent)
	wantFg(t, styledLine(t, v, `"name": "web-demo"`), `"web-demo"`, fgGood)
	wantFg(t, styledLine(t, v, `"cost": "42"`), `"cost"`, fgAccent)
}
