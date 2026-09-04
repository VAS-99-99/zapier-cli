package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"
)

func nestedOutputFixture(t *testing.T) json.RawMessage {
	t.Helper()
	b, err := json.Marshal([]map[string]any{{
		"nested": map[string]any{"z": "quoted \"value\"\nline", "a": 1},
		"tags":   []any{"one", "two"},
		"quoted": "a,\"b\"",
		"text":   "tab\tand\nnewline",
	}})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestNestedCSVCellsAreCompactJSONAndQuoted(t *testing.T) {
	var out bytes.Buffer
	if err := printCSV(&out, nestedOutputFixture(t)); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(strings.NewReader(out.String())).ReadAll()
	if err != nil {
		t.Fatalf("CSV must preserve quoting and newlines: %v", err)
	}
	want := [][]string{{"nested", "quoted", "tags", "text"}, {
		`{"a":1,"z":"quoted \"value\"\nline"}`, `a,"b"`, `["one","two"]`, "tab\tand\nnewline",
	}}
	if !equalCSVRows(rows, want) {
		t.Fatalf("CSV rows = %#v, want %#v", rows, want)
	}
}

func equalCSVRows(got, want [][]string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if strings.Join(got[i], "\x00") != strings.Join(want[i], "\x00") {
			return false
		}
	}
	return true
}

func TestNestedPlainCellsAreCompactJSONAndNormalized(t *testing.T) {
	var out bytes.Buffer
	if err := printPlain(&out, nestedOutputFixture(t)); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{
		"nested\tquoted\ttags\ttext\n",
		`{"a":1,"z":"quoted \"value\"\nline"}`,
		`["one","two"]`,
		"a,\"b\"",
		"tab and newline",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("plain output missing %q:\n%s", want, got)
		}
	}
	if strings.Count(got, "\n") != 2 {
		t.Fatalf("plain output leaked a cell newline: %q", got)
	}
}
