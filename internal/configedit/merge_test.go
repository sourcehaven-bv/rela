package configedit

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func readTree(t *testing.T, src []byte) any {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal(src, &doc); err != nil {
		t.Fatal(err)
	}
	tree, err := FromYAML(&doc)
	if err != nil {
		t.Fatal(err)
	}
	return tree
}

// wire sends a tree through its JSON form and back, as the browser would.
func wire(t *testing.T, tree any) any {
	t.Helper()
	raw, err := json.Marshal(tree)
	if err != nil {
		t.Fatal(err)
	}
	back, err := DecodeJSON(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	return back
}

func child(t *testing.T, tree any, path ...string) *Map {
	t.Helper()
	cur := tree
	for _, key := range path {
		m, ok := cur.(*Map)
		if !ok {
			t.Fatalf("%s: not a mapping", key)
		}
		v, ok := m.Get(key)
		if !ok {
			t.Fatalf("missing %s", key)
		}
		cur = v
	}
	m, ok := cur.(*Map)
	if !ok {
		t.Fatalf("%v: not a mapping", path)
	}
	return m
}

// changedLines lists the lines of a unified comparison that differ, as
// "-old" / "+new", using the same matcher the merge uses.
func changedLines(before, after string) []string {
	a, b := splitLines(before), splitLines(after)
	m, _ := matchLines(a, b)
	var out []string
	inB := make([]bool, len(b))
	for i, j := range m {
		if j < 0 {
			out = append(out, "-"+strings.TrimRight(a[i], "\n"))
		} else {
			inB[j] = true
		}
	}
	for j, ok := range inB {
		if !ok {
			out = append(out, "+"+strings.TrimRight(b[j], "\n"))
		}
	}
	return out
}

func TestApply_UnchangedTreeIsByteIdentical(t *testing.T) {
	for _, name := range []string{"testdata/schema.yaml", "testdata/data-entry.yaml"} {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(name)
			if err != nil {
				t.Fatal(err)
			}
			out, err := Apply(src, wire(t, readTree(t, src)))
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out, src) {
				t.Fatalf("unchanged tree rewrote the file: %v", changedLines(string(src), string(out)))
			}
		})
	}
}

func TestApply_EditsTouchOnlyTheirLines(t *testing.T) {
	src, err := os.ReadFile("testdata/schema.yaml")
	if err != nil {
		t.Fatal(err)
	}
	dataEntry, err := os.ReadFile("testdata/data-entry.yaml")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		file []byte
		edit func(t *testing.T, tree any)
		want []string
		// alt is an equally short answer: a swap of two items reports either one as moved.
		alt []string
	}{
		{
			name: "change a scalar keeps its line comment and quoting",
			file: src,
			edit: func(t *testing.T, tree any) {
				t.Helper()
				child(t, tree, "entities", "ticket").Set("id_prefix", "TK-")
			},
			want: []string{`-    id_prefix: "TKT-"`, `+    id_prefix: "TK-"`},
		},
		{
			name: "add an option to a flow list",
			file: src,
			edit: func(t *testing.T, tree any) {
				t.Helper()
				ts := child(t, tree, "types", "ticket_status")
				v, _ := ts.Get("values")
				ts.Set("values", append(v.([]any), "on-hold"))
			},
			want: []string{
				"-    values: [backlog, ready, planning, in-progress, review, done, wont-fix, blocked]",
				"+    values: [backlog, ready, planning, in-progress, review, done, wont-fix, blocked, on-hold]",
			},
		},
		{
			name: "reorder two properties moves only their lines",
			file: src,
			edit: func(t *testing.T, tree any) {
				t.Helper()
				props := child(t, tree, "entities", "ticket", "properties")
				i, j := indexOf(props.Keys, "priority"), indexOf(props.Keys, "effort")
				props.Keys[i], props.Keys[j] = props.Keys[j], props.Keys[i]
				props.Values[i], props.Values[j] = props.Values[j], props.Values[i]
			},
			want: []string{"-      effort:", "-        type: effort", "+      effort:", "+        type: effort"},
			alt:  []string{"-      priority:", "-        type: priority", "+      priority:", "+        type: priority"},
		},
		{
			name: "add a property",
			file: src,
			edit: func(t *testing.T, tree any) {
				t.Helper()
				props := child(t, tree, "entities", "ticket", "properties")
				due := &Map{}
				due.Set("type", "date")
				props.Set("due", due)
			},
			want: []string{"+      due:", "+        type: date"},
		},
		{
			name: "remove a form field",
			file: dataEntry,
			edit: func(t *testing.T, tree any) {
				t.Helper()
				form := child(t, tree, "forms", "edit_idea")
				f, _ := form.Get("fields")
				fields := f.([]any)
				form.Set("fields", append(fields[:4:4], fields[5:]...))
			},
			want: []string{"-      - property: effort"},
		},
		{
			name: "add a color for a new option",
			file: dataEntry,
			edit: func(t *testing.T, tree any) {
				t.Helper()
				child(t, tree, "styles", "ticket_status").Set("on-hold", "orange")
			},
			want: []string{"+    on-hold: orange"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := wire(t, readTree(t, tc.file))
			tc.edit(t, tree)
			out, err := Apply(tc.file, wire(t, tree))
			if err != nil {
				t.Fatal(err)
			}
			got := changedLines(string(tc.file), string(out))
			if strings.Join(got, "\n") != strings.Join(tc.want, "\n") &&
				(tc.alt == nil || strings.Join(got, "\n") != strings.Join(tc.alt, "\n")) {

				t.Fatalf("changed lines:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(tc.want, "\n"))
			}
			// The result must still parse to the edited tree.
			if a, b := mustJSON(t, stripOrigins(readTree(t, out))), mustJSON(t, stripOrigins(tree)); a != b {
				t.Fatal("written file does not read back as the edited tree")
			}
		})
	}
}

func indexOf(keys []string, k string) int {
	for i, key := range keys {
		if key == k {
			return i
		}
	}
	return -1
}

// stripOrigins clears list origins, which renumber when an item is removed.
func stripOrigins(v any) any {
	switch t := v.(type) {
	case *Map:
		t.HasOrigin, t.Origin = false, 0
		for _, c := range t.Values {
			stripOrigins(c)
		}
	case []any:
		for _, c := range t {
			stripOrigins(c)
		}
	}
	return v
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestApply_OriginKeepsARenamedItemsComments(t *testing.T) {
	src := []byte("nav:\n  # first entry\n  - label: Tickets\n    list: all\n  # second entry\n  - label: Ideas\n    list: ideas\n")
	tree := wire(t, readTree(t, src))
	nav, _ := tree.(*Map).Get("nav")
	list := nav.([]any)
	list[0].(*Map).Set("label", "Work")
	list[0], list[1] = list[1], list[0]
	out, err := Apply(src, wire(t, tree))
	if err != nil {
		t.Fatal(err)
	}
	want := "nav:\n  # second entry\n  - label: Ideas\n    list: ideas\n  # first entry\n  - label: Work\n    list: all\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestDecodeJSON_Rejects(t *testing.T) {
	for name, in := range map[string]string{
		"plain object":    `{"a": 1}`,
		"duplicate key":   `{"$m": [["a", 1], ["a", 2]]}`,
		"bad pair":        `{"$m": [["a"]]}`,
		"trailing data":   `[] []`,
		"extra map key":   `{"$m": [], "b": 1}`,
		"non-string key":  `{"$m": [[1, 2]]}`,
		"negative origin": `{"$m": [], "$i": -1}`,
		"string origin":   `{"$m": [], "$i": "0"}`,
		"origin only":     `{"$i": 0}`,
		"too deep":        strings.Repeat("[", maxDepth+2) + strings.Repeat("]", maxDepth+2),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := DecodeJSON(strings.NewReader(in)); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestFromYAML_RejectsAliases(t *testing.T) {
	for name, in := range map[string]string{
		"alias":     "a: &x 1\nb: *x\n",
		"merge key": "a: &x {c: 1}\nb:\n  <<: *x\n",
	} {
		t.Run(name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(in), &doc); err != nil {
				t.Fatal(err)
			}
			if _, err := FromYAML(&doc); err == nil {
				t.Fatal("accepted")
			}
		})
	}
}

func TestApply_NewStringsStayStrings(t *testing.T) {
	m := &Map{}
	for _, s := range []string{"yes", "1.0", "null", "2026-01-01", "~", "0x1F", "true", "a: b", "- x", "#c"} {
		m.Set("k"+s, s)
	}
	out, err := Apply([]byte("a: 1\n"), m)
	if err != nil {
		t.Fatal(err)
	}
	var back map[string]any
	if err := yaml.Unmarshal(out, &back); err != nil {
		t.Fatal(err)
	}
	for i, k := range m.Keys {
		if back[k] != m.Values[i] {
			t.Errorf("%s: read back %#v (%T)", k, back[k], back[k])
		}
	}
}

// TestApply_WritesWhatWasChecked pins that the bytes Apply writes always hold
// the edited tree, also when the file's layout differs from the encoder's
// and the line merge would pair lines at the wrong depth.
func TestApply_WritesWhatWasChecked(t *testing.T) {
	var long strings.Builder
	for i := range 2500 {
		fmt.Fprintf(&long, "k%d:\n    v: %d\n", i, i)
	}
	tests := []struct {
		name string
		src  string
	}{
		{"four-space indent", "x:\n    c: 1\ny:\n    z:\n        c: 1\nw: 2\n"},
		{"indentless list", "x:\n- a: 1\n- b: 2\nw: 2\n"},
		{"beyond the edit bound", long.String() + "w: 2\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tree := readTree(t, []byte(tc.src)).(*Map)
			tree.Set("w", "9")
			out, err := Apply([]byte(tc.src), wire(t, tree))
			if err != nil {
				t.Fatal(err)
			}
			got := readTree(t, out)
			if !reflect.DeepEqual(stripped(got), stripped(tree)) {
				t.Fatalf("written file does not hold the edited tree:\n%s", out)
			}
		})
	}
}

func TestFromYAML_NonFiniteFloatsStayText(t *testing.T) {
	tree := readTree(t, []byte("a: .inf\nb: -.Inf\nc: .nan\n"))
	if _, err := json.Marshal(tree); err != nil {
		t.Fatalf("snapshot of a file with .inf/.nan does not encode: %v", err)
	}
}

// TestMergeKeysAreRefused pins both layers against a "<<" key: the wire
// decoder refuses it, and Apply refuses a tree that does not read back as
// itself, which is what a merge key would do.
func TestMergeKeysAreRefused(t *testing.T) {
	if _, err := DecodeJSON(strings.NewReader(`{"$m":[["<<",{"$m":[["scan","off"]]}]]}`)); err == nil {
		t.Fatal("decoder accepted a merge key")
	}
	tree := &Map{}
	tree.Set("lists", &Map{Keys: []string{"<<"}, Values: []any{
		&Map{Keys: []string{"all"}, Values: []any{&Map{Keys: []string{"export_render"}, Values: []any{"x.lua"}}}},
	}})
	if _, err := Apply([]byte("lists: {}\n"), tree); !errors.Is(err, ErrBadDraft) {
		t.Fatalf("Apply wrote a merge key: %v", err)
	}
}
