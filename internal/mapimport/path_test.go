package mapimport

import (
	"strings"
	"testing"
)

const pathDoc = `{
  "catalog": {
    "groups": [
      {"id": "g1", "class": "category", "controls": [
        {"id": "c1", "props": [{"name": "label", "value": "1"}, {"name": "sort", "value": "a"}],
         "controls": [{"id": "c1.1"}]},
        {"id": "c2"}
      ]},
      {"id": "g2", "controls": [{"id": "c3", "props": [{"name": "label", "value": "3"}]}]}
    ]
  }
}`

func TestPath_Eval(t *testing.T) {
	root, err := Decode([]byte(pathDoc), FormatJSON, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	c1 := (&Path{absolute: true, steps: []step{
		{kind: stepField, name: "catalog"}, {kind: stepField, name: "groups"}, {kind: stepField, name: "controls"},
	}}).Eval(root, root)[0]
	cases := []struct {
		path string
		cur  *Node
		want string
	}{
		{"$.catalog.groups.id", nil, "g1,g2"},
		{"$.catalog..controls.id", nil, "c1,c1.1,c2,c3"},
		{"$.catalog.groups[class=category].id", nil, "g1"},
		{"$.catalog.groups[class='category'].id", nil, "g1"},
		{"$.catalog.groups.controls[props].id", nil, "c1,c3"},
		{"$.catalog.groups.*", nil, "g1,category,g2"}, // the controls are structures, not values
		{"props[name=label].value", c1, "1"},
		{"props[name=\"sort\"].value", c1, "a"},
		{"^.id", c1, "g1"},
		{"controls.^.id", c1, "c1"},
		{"id", c1, "c1"},
		{"$..groups..controls.id", nil, "c1,c1.1,c2,c3"}, // each control once
		{"$.catalog..controls..controls.id", nil, "c1.1"},
		{"['id']", c1, "c1"},
		{"props[name='la]bel'].value", c1, ""},
		{".", c1, ""},
		{"missing", c1, ""},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			p, err := CompilePath(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			cur := tc.cur
			if cur == nil {
				cur = root
			}
			var got []string
			for _, n := range p.Eval(cur, root) {
				if s, ok := n.text(); ok {
					got = append(got, s)
				}
			}
			if strings.Join(got, ",") != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCompilePath_Errors(t *testing.T) {
	for _, src := range []string{
		"", "$x", ".a", "a.", "a..", "a.[b]", "[b]", "a[b", "a[=x]", "a[b='x]", "a b", "a.(b)", "a..[x]",
		"['a", "['a'", "['']", "a['b']c",
	} {
		t.Run(src, func(t *testing.T) {
			if _, err := CompilePath(src); err == nil {
				t.Fatalf("CompilePath(%q) accepted it", src)
			}
		})
	}
}

func TestTemplate(t *testing.T) {
	root, err := Decode([]byte(`{"a": "x", "b": ["1", "2"], "c": {"d": 1}}`), FormatJSON, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	sep := "+"
	cases := []struct {
		src     string
		join    *string
		want    string
		wantErr string
	}{
		{"{{a}}-{{ a }}", nil, "x-x", ""},
		{"{{b}}", &sep, "1+2", ""},
		{"{{b}}", nil, "", "matched 2 values"},
		{"{{c}}", nil, "", "a structure"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			tm, err := compileTemplate(tc.src)
			if err != nil {
				t.Fatal(err)
			}
			got, err := tm.render(root, root, tc.join)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("render = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	for _, bad := range []string{"plain", "{{a", "{{$x}}"} {
		if _, err := compileTemplate(bad); err == nil {
			t.Errorf("compileTemplate(%q) accepted it", bad)
		}
	}
}
