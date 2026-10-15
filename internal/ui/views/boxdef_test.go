package views

import (
	"reflect"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

func TestFormats(t *testing.T) {
	list := []any{"a", "b"}
	tests := []struct {
		name   string
		format format
		in     any
		want   string
	}{
		{"text", fmtText, "abc", "abc"},
		{"text of a number", fmtText, 5.0, "5"},
		{"text missing", fmtText, nil, dash},
		{"text of a flag", fmtText, true, dash},
		{"text empty", fmtText, "", dash},
		{"state", fmtState, "Succeeded", "● Succeeded"},
		{"state missing", fmtState, nil, dash},
		{"on", fmtOnOff, true, "on"},
		{"off", fmtOnOff, false, "off"},
		{"on/off missing", fmtOnOff, nil, dash},
		{"on/off of a string", fmtOnOff, "true", dash},
		{"on/off missing, default on", fmtOnOffOr(true), nil, "on"},
		{"on/off missing, default off", fmtOnOffOr(false), nil, "off"},
		{"on/off set off, default on", fmtOnOffOr(true), false, "off"},
		{"on/off set on, default off", fmtOnOffOr(false), true, "on"},
		{"on/off of a string, default on", fmtOnOffOr(true), "false", "on"},
		{"enabled", fmtEnabled, "enabled", "on"},
		{"disabled", fmtEnabled, "disabled", "off"},
		{"enabled in upper case", fmtEnabled, "ENABLED", "on"},
		{"disabled in mixed case", fmtEnabled, "Disabled", "off"},
		{"enabled bool", fmtEnabled, true, "on"},
		{"disabled bool", fmtEnabled, false, "off"},
		{"enabled other text", fmtEnabled, "auto", "auto"},
		{"enabled missing", fmtEnabled, nil, dash},
		{"enabled empty", fmtEnabled, "", dash},
		{"date", fmtDate, "2024-03-05T10:20:30Z", "2024-03-05"},
		{"date with fraction", fmtDate, "2024-03-05T10:20:30.1234567Z", "2024-03-05"},
		{"date in UTC", fmtDate, "2024-03-05T23:30:00-02:00", "2024-03-06"},
		{"date of other text", fmtDate, "last week", "last week"},
		{"date missing", fmtDate, nil, dash},
		{"date empty", fmtDate, "", dash},
		{"count of a list", fmtCount, list, "2"},
		{"count of a map", fmtCount, map[string]any{"x": 1.0, "y": 2.0, "z": 3.0}, "3"},
		{"count of an empty list", fmtCount, []any{}, "0"},
		{"count missing", fmtCount, nil, "0"},
		{"count of a string", fmtCount, "abc", "0"},
		{"last segment", fmtLast, "/subscriptions/s/resourceGroups/rg/id-one", "id-one"},
		{"last of a plain text", fmtLast, "one", "one"},
		{"last missing", fmtLast, nil, dash},
		{"join", fmtJoin, list, "a, b"},
		{"join of numbers", fmtJoin, []any{1.0, "x"}, "1, x"},
		{"join empty", fmtJoin, []any{}, dash},
		{"join missing", fmtJoin, nil, dash},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := plain(tt.format(tt.in)); got != tt.want {
				t.Errorf("format(%v) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestFormatColors(t *testing.T) {
	wantFg(t, fmtState("Succeeded"), "Succeeded", fgGood)
	wantFg(t, fmtState("Failed"), "●", fgBad)
	wantFg(t, fmtOnOff(true), "on", fgGood)
	wantFg(t, fmtOnOff(false), "off", fgMuted)
	wantFg(t, fmtOnOff(nil), dash, "")
	wantFg(t, fmtOnOffOr(true)(nil), "on", fgGood)
	wantFg(t, fmtOnOffOr(false)(nil), "off", fgMuted)
	wantFg(t, fmtEnabled("enabled"), "on", fgGood)
	wantFg(t, fmtEnabled("disabled"), "off", fgMuted)
	wantFg(t, fmtEnabled(nil), dash, "")
}

func TestFieldDef(t *testing.T) {
	row := map[string]any{
		"properties": map[string]any{"vaultUri": "https://v", "flag": true, "ids": []any{"x", "y"}},
		"location":   "westeurope",
	}
	tests := []struct {
		name string
		f    fieldDef
		want string
	}{
		{"path and format", fieldDef{label: "URI", path: []string{"properties", "vaultUri"}, format: fmtText}, "https://v"},
		{"nested format", fieldDef{path: []string{"properties", "flag"}, format: fmtOnOff}, "on"},
		{"list format", fieldDef{path: []string{"properties", "ids"}, format: fmtJoin}, "x, y"},
		{"no format means text", fieldDef{path: []string{"location"}}, "westeurope"},
		{"missing path", fieldDef{path: []string{"properties", "nope"}, format: fmtText}, dash},
		{"value func", fieldDef{value: func(r map[string]any) string { return str(r, "location") + "!" }}, "westeurope!"},
		{"value func wins over the path", fieldDef{path: []string{"location"}, value: func(map[string]any) string { return "v" }}, "v"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := plain(tt.f.text(row)); got != tt.want {
				t.Errorf("text = %q, want %q", got, tt.want)
			}
		})
	}
}

func fiveFields() boxDef {
	b := boxDef{title: "Five", split: true}
	for _, k := range []string{"a", "b", "c", "d", "e"} {
		b.fields = append(b.fields, fieldDef{label: k, path: []string{k}})
	}
	return b
}

var fiveRow = map[string]any{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5"}

func TestBoxLinesOneColumn(t *testing.T) {
	want := []string{"a  1", "b  2", "c  3", "d  4", "e  5"}
	for _, width := range []int{89, 50, 0} {
		if got := plainAll(fiveFields().lines(fiveRow, width)); !reflect.DeepEqual(got, want) {
			t.Errorf("width %d: lines = %q, want %q", width, got, want)
		}
	}
	b := fiveFields()
	b.split = false
	if got := plainAll(b.lines(fiveRow, 200)); !reflect.DeepEqual(got, want) {
		t.Errorf("without split: lines = %q, want %q", got, want)
	}
}

func TestBoxLinesTwoColumnsSplitOddCounts(t *testing.T) {
	// Width 90: columns of 43 and a gap of 4; the left has 3 fields, the right 2.
	want := []string{twoCol("a  1", "d  4", 43), twoCol("b  2", "e  5", 43), twoCol("c  3", "", 43)}
	if got := plainLines(strings.Join(fiveFields().lines(fiveRow, 90), "\n")); !reflect.DeepEqual(got, want) {
		t.Errorf("lines = %q, want %q", got, want)
	}
}

func TestBoxLinesTwoColumnsOfOneField(t *testing.T) {
	b := boxDef{split: true, fields: []fieldDef{{label: "a", path: []string{"a"}}}}
	got := plainAll(b.lines(fiveRow, 100))
	if len(got) != 1 || strings.TrimRight(got[0], " ") != "a  1" {
		t.Errorf("lines = %q", got)
	}
}

func TestBoxExtraPairsComeFirst(t *testing.T) {
	b := boxDef{
		extra:  func(map[string]any) []pair { return []pair{{"zz", "9"}} },
		fields: []fieldDef{{label: "a", path: []string{"a"}}},
	}
	want := []string{"zz  9", "a   1"}
	if got := plainAll(b.lines(fiveRow, 50)); !reflect.DeepEqual(got, want) {
		t.Errorf("lines = %q, want %q", got, want)
	}
}

func TestBoxFieldWithShowIsDrawnOnlyWhenShown(t *testing.T) {
	only := func(v string) func(map[string]any) bool {
		return func(row map[string]any) bool { return str(row, "a") == v }
	}
	b := boxDef{fields: []fieldDef{
		{label: "a", path: []string{"a"}},
		{label: "yes", path: []string{"a"}, show: only("1")},
		{label: "no", path: []string{"a"}, show: only("2")},
	}}
	want := []string{"a    1", "yes  1"}
	if got := plainAll(b.lines(fiveRow, 50)); !reflect.DeepEqual(got, want) {
		t.Errorf("lines = %q, want %q", got, want)
	}
}

func TestBoxesTabDrawsEachBoxAsASection(t *testing.T) {
	boxes := []boxDef{
		{title: "One", fields: []fieldDef{{label: "a", path: []string{"a"}}}},
		{title: "Two", fields: []fieldDef{{label: "b", path: []string{"b"}}}},
	}
	got := unbox(plainLines(strings.Join(boxesTab(boxes...)(fiveRow, 40), "\n")))
	want := []string{"One", "a  1", "", "Two", "b  2", ""}
	wantLines(t, got, want)
	if narrow := boxesTab(boxes...)(fiveRow, 10); !reflect.DeepEqual(plainAll(narrow)[:2], []string{"One", "a  1"}) {
		t.Errorf("narrow = %q", plainAll(narrow))
	}
}

func overviewOf(render func(map[string]any, int) []string, attn func(map[string]any) []alert, row map[string]any) []string {
	return unbox(plainLines(strings.Join(withTail(render, attn)(row, 60), "\n")))
}

func TestWithTail(t *testing.T) {
	body := boxesTab(boxDef{title: "One", fields: []fieldDef{{label: "a", path: []string{"a"}}}})
	row := map[string]any{"a": "1", "tags": map[string]any{"k": "v"}}
	warnOne := func(map[string]any) []alert { return []alert{{text: "careful"}} }
	none := func(map[string]any) []alert { return nil }
	tests := []struct {
		name string
		attn func(map[string]any) []alert
		want []string
	}{
		{"no rules", nil, []string{"One", "a  1", "", "Tags", "k = v", ""}},
		{"rules without alerts", none, []string{"One", "a  1", "", "Tags", "k = v", ""}},
		{"alerts", warnOne, []string{"One", "a  1", "", "Attention", "▲ careful", "", "Tags", "k = v", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { wantLines(t, overviewOf(body, tt.attn, row), tt.want) })
	}
}

func TestWithTailTagsWithoutTags(t *testing.T) {
	body := boxesTab(boxDef{title: "One", fields: []fieldDef{{label: "a", path: []string{"a"}}}})
	wantLines(t, overviewOf(body, nil, fiveRow), []string{"One", "a  1", "", "Tags", "none", ""})
}

func TestTabDefsAddAttentionAndTagsToTheOverviewOnly(t *testing.T) {
	defs := tabDefs("microsoft.web/sites")
	if len(defs) != 2 || defs[0].name != overviewTab || defs[1].name != "JSON" {
		t.Fatalf("defs = %v", defs)
	}
	json := defs[1].render(map[string]any{"a": 1.0}, 40)
	if strings.Contains(strings.Join(plainAll(json), "\n"), "Tags") {
		t.Errorf("the JSON tab has tags: %q", json)
	}
}

func TestBoxLinesTakeOneColumnRatherThanCut(t *testing.T) {
	long := strings.Repeat("x", 50)
	for _, k := range []string{"a", "d"} { // a value in the left and in the right column
		row := map[string]any{"a": "1", "b": "2", "c": "3", "d": "4", "e": "5"}
		row[k] = long
		var want []string
		for _, f := range []string{"a", "b", "c", "d", "e"} {
			want = append(want, f+"  "+row[f].(string))
		}
		if got := plainAll(fiveFields().lines(row, 90)); !reflect.DeepEqual(got, want) {
			t.Errorf("long %s: lines = %q, want %q", k, got, want)
		}
	}
}

func TestBoxWithoutPairsSaysNone(t *testing.T) {
	for _, split := range []bool{false, true} {
		got := boxDef{title: "Empty", split: split}.lines(nil, 120)
		if !reflect.DeepEqual(got, []string{"none"}) {
			t.Errorf("split %v: lines = %q, want none", split, got)
		}
	}
}

// tableRow has maps and plain strings to draw as table elements.
var tableRow = map[string]any{
	"things": []any{
		map[string]any{"name": "alpha", "meta": map[string]any{"id": "/x/y/ref-one"}, "up": true, "tags": []any{"t1", "t2"}, "state": "Running"},
		map[string]any{"name": "beta", "meta": map[string]any{"id": "/x/y/ref-two"}, "up": false, "tags": []any{"t3"}, "state": "Failed"},
	},
	"words": []any{"pear", "fig", "apple"},
}

func TestTableDefColumnsAndLayout(t *testing.T) {
	tb := tableDef{path: []string{"things"}, cols: []colDef{
		{title: "Name", path: []string{"name"}},
		{title: "Ref", path: []string{"meta", "id"}, format: fmtLast},
		{title: "Shout", value: func(e any) string { return strings.ToUpper(str(e, "name")) }},
	}}
	got := plainAll(tb.lines(tableRow, 200))
	// Each column as wide as its widest cell, two spaces between, none after the last.
	want := []string{"Name   Ref      Shout", "alpha  ref-one  ALPHA", "beta   ref-two  BETA"}
	wantLines(t, got, want)
	raw := tb.lines(tableRow, 200)
	if a := attrsOf(t, raw[0], "Name"); !a.bold || a.fg != fgMuted {
		t.Errorf("header = %+v, want bold muted", a)
	}
	for i, l := range raw {
		if strings.Contains(l, "\x1b[7m") {
			t.Errorf("line %d is highlighted", i)
		}
	}
}

func TestTableDefFormats(t *testing.T) {
	tb := tableDef{path: []string{"things"}, cols: []colDef{
		{title: "Name", path: []string{"name"}},
		{title: "Up", path: []string{"up"}, format: fmtOnOff},
		{title: "Tags", path: []string{"tags"}, format: fmtCount},
		{title: "List", path: []string{"tags"}, format: fmtJoin},
		{title: "State", path: []string{"state"}, format: fmtState},
		{title: "Gone", path: []string{"nope"}},
	}}
	raw := tb.lines(tableRow, 200)
	wantLines(t, plainAll(raw), []string{
		"Name   Up   Tags  List    State      Gone",
		"alpha  on   2     t1, t2  ● Running  —",
		"beta   off  1     t3      ● Failed   —",
	})
	wantFg(t, raw[1], "on", fgGood)
	wantFg(t, raw[2], "off", fgMuted)
	wantFg(t, raw[1], "Running", fgGood)
	wantFg(t, raw[2], "Failed", fgBad)
}

func TestTableDefPlainStringElements(t *testing.T) {
	tb := tableDef{path: []string{"words"}, cols: []colDef{
		{title: "Word", value: text},
		{title: "Size", value: func(e any) string { return strconv.Itoa(len(text(e))) }},
		{title: "Path", path: []string{"x"}},
		{title: "Bare", format: fmtText},
	}}
	wantLines(t, plainAll(tb.lines(tableRow, 200)), []string{
		"Word   Size  Path  Bare",
		"pear   4     —     pear",
		"fig    3     —     fig",
		"apple  5     —     apple",
	})
}

func TestTableDefKeepAndOrder(t *testing.T) {
	word := []colDef{{title: "Word", value: text}}
	byLength := func(a, b any) int { return len(text(a)) - len(text(b)) }
	tests := []struct {
		name string
		tb   tableDef
		want []string
	}{
		{"list order by default", tableDef{path: []string{"words"}, cols: word}, []string{"Word", "pear", "fig", "apple"}},
		{"keep drops", tableDef{path: []string{"words"}, cols: word, keep: func(e any) bool { return text(e) != "fig" }}, []string{"Word", "pear", "apple"}},
		{"order sorts", tableDef{path: []string{"words"}, cols: word, order: byLength}, []string{"Word", "fig", "pear", "apple"}},
		{"order sorts the kept ones", tableDef{path: []string{"words"}, cols: word, order: byLength, keep: func(e any) bool { return text(e) != "fig" }}, []string{"Word", "pear", "apple"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { wantLines(t, plainAll(tt.tb.lines(tableRow, 200)), tt.want) })
	}
	if got := tableRow["words"].([]any); !reflect.DeepEqual(got, []any{"pear", "fig", "apple"}) {
		t.Errorf("the row's list was changed: %v", got)
	}
}

func TestTableDefOrderIsStable(t *testing.T) {
	// Enough elements that an unstable sort would move equal ones.
	var list []any
	var want []string
	for _, parity := range []int{0, 1} {
		for i := parity; i < 60; i += 2 {
			want = append(want, strconv.Itoa(i))
		}
	}
	for i := range 60 {
		list = append(list, strconv.Itoa(i))
	}
	odd := func(e any) int { n, _ := strconv.Atoi(text(e)); return n % 2 }
	tb := tableDef{path: []string{"l"}, cols: []colDef{{title: "W", value: text}},
		order: func(a, b any) int { return odd(a) - odd(b) }}
	wantLines(t, plainAll(tb.lines(map[string]any{"l": list}, 200)), append([]string{"W"}, want...))
}

func TestTableDefEmpty(t *testing.T) {
	col := []colDef{{title: "Word", value: text}}
	dropAll := func(any) bool { return false }
	tests := []struct {
		name string
		tb   tableDef
		row  map[string]any
		want string
	}{
		{"default text, no list", tableDef{path: []string{"nope"}, cols: col}, tableRow, "none"},
		{"default text, empty list", tableDef{path: []string{"e"}, cols: col}, map[string]any{"e": []any{}}, "none"},
		{"default text, nothing kept", tableDef{path: []string{"words"}, cols: col, keep: dropAll}, tableRow, "none"},
		{"own text", tableDef{path: []string{"words"}, cols: col, keep: dropAll, empty: "no words"}, tableRow, "no words"},
		{"own text, no list", tableDef{path: []string{"nope"}, cols: col, empty: "no words"}, tableRow, "no words"},
		{"not a list", tableDef{path: []string{"name"}, cols: col}, map[string]any{"name": "x"}, "none"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { wantLines(t, plainAll(tt.tb.lines(tt.row, 200)), []string{tt.want}) })
	}
}

func TestBoxDefWithATable(t *testing.T) {
	tb := &tableDef{path: []string{"words"}, cols: []colDef{{title: "Word", value: text}}}
	b := boxDef{title: "Words", table: tb}
	wantLines(t, plainAll(b.lines(tableRow, 80)), []string{"Word", "pear", "fig", "apple"})
	wantLines(t, unbox(plainLines(strings.Join(boxesTab(b)(tableRow, 40), "\n"))), []string{"Words", "Word", "pear", "fig", "apple", ""})
	if got := plainAll(b.asSection(tableRow, 40).lines); got[0] != "Word" {
		t.Errorf("section lines = %q", got)
	}
}

func TestBoxDefTableKeepsItsLastColumnInTheBox(t *testing.T) {
	row := map[string]any{"l": []any{"a-long-cell-that-does-not-fit-into-the-box"}}
	tb := &tableDef{path: []string{"l"}, cols: []colDef{
		{title: "A", value: text},
		{title: "B", value: func(any) string { return "b" }},
	}}
	got := boxesTab(boxDef{title: "T", table: tb})(row, 24)
	for _, l := range plainAll(got) {
		if w := lipgloss.Width(l); w > 24 {
			t.Errorf("line %q is %d cells wide, want at most 24", l, w)
		}
	}
	if line := plainAll(got)[2]; !strings.Contains(line, "…") || !strings.HasSuffix(strings.TrimSuffix(line, " │"), "  b") {
		t.Errorf("row = %q, want the cell cut with an ellipsis and b whole", line)
	}
}

func TestTableDefListReplacesPath(t *testing.T) {
	list := func(row map[string]any) []any {
		return []any{map[string]any{"n": str(row, "x") + "-1"}, map[string]any{"n": str(row, "x") + "-2"}}
	}
	cols := []colDef{{title: "N", path: []string{"n"}}}
	row := map[string]any{"x": "a", "words": []any{"ignored"}}
	wantLines(t, plainAll(tableDef{path: []string{"words"}, list: list, cols: cols}.lines(row, 200)), []string{"N", "a-1", "a-2"})
	wantLines(t, plainAll(tableDef{list: list, cols: cols, keep: func(e any) bool { return str(e, "n") != "a-1" }}.lines(row, 200)), []string{"N", "a-2"})
	wantLines(t, plainAll(tableDef{list: func(map[string]any) []any { return nil }, cols: cols, empty: "nothing"}.lines(row, 200)), []string{"nothing"})
}
