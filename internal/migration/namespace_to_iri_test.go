package migration

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNamespaceToIRIMigration(t *testing.T) {
	tests := []struct {
		name       string
		in         string
		wantDetect bool
		wantOut    string
	}{
		{"iri value", "version: \"1.0\"\nnamespace: \"https://example.org/x#\"\n", true, "iri: \"https://example.org/x#\""},
		{"namespace name in an included file", "namespace: isms\n", false, ""},
		{"already migrated", "iri: \"https://example.org/x#\"\n", false, ""},
		{"no key", "version: \"1.0\"\n", false, ""},
	}
	m := &NamespaceToIRIMigration{}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tc.in), &doc); err != nil {
				t.Fatal(err)
			}
			if got := m.Detect(&doc); got != tc.wantDetect {
				t.Fatalf("Detect = %v, want %v", got, tc.wantDetect)
			}
			if !tc.wantDetect {
				return
			}
			if err := m.Apply(&doc); err != nil {
				t.Fatal(err)
			}
			out, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(out), tc.wantOut) || strings.Contains(string(out), "namespace:") {
				t.Fatalf("Apply output:\n%s", out)
			}
		})
	}
}

// TestBlocking_SkipsAdvisory pins that the namespace rename does not stop a
// project from loading: the loader still reads the old key.
func TestBlocking_SkipsAdvisory(t *testing.T) {
	detections := []DetectionResult{{Migration: &NamespaceToIRIMigration{}}}
	if got := Blocking(detections); len(got) != 0 {
		t.Errorf("Blocking = %v, want none", got)
	}
}
