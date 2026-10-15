package views

import (
	"reflect"
	"strings"
	"testing"
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
