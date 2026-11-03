package yamlscalar

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestPlainYAML11Bool(t *testing.T) {
	tests := []struct {
		name string
		doc  string
		want bool
	}{
		{"plain yes", "v: yes", true},
		{"plain No", "v: No", true},
		{"plain ON", "v: ON", true},
		{"plain off", "v: off", true},
		{"plain y", "v: y", true},
		{"plain N", "v: N", true},
		{"literal block yes", "v: |\n  yes\n", true},
		{"stripped block yes", "v: |-\n  yes\n", true},
		{"folded block off", "v: >\n  off\n", true},
		{"double quoted yes", `v: "yes"`, false},
		{"single quoted no", `v: 'no'`, false},
		{"tagged str yes", "v: !!str yes", false},
		{"long tagged str on", "v: !<tag:yaml.org,2002:str> on", false},
		{"other tag yes", "v: !custom yes", true},
		{"true", "v: true", false},
		{"false", "v: false", false},
		{"text", "v: yesterday", false},
		{"number", "v: 1", false},
		{"mapping", "v: {a: 1}", false},
		{"list", "v: [yes]", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tc.doc), &doc); err != nil {
				t.Fatal(err)
			}
			v := doc.Content[0].Content[1]
			if got := PlainYAML11Bool(v); got != tc.want {
				t.Errorf("PlainYAML11Bool(%q) = %v, want %v", tc.doc, got, tc.want)
			}
		})
	}
}

func TestPlainYAML11Bool_Nil(t *testing.T) {
	if PlainYAML11Bool(nil) {
		t.Error("nil node reported as a boolean")
	}
}
