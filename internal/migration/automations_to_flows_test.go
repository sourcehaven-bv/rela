package migration

import (
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func TestAutomationsToFlows_Detect(t *testing.T) {
	tests := []struct {
		name, input string
		want        bool
	}{
		{"non-empty", "automations:\n  - name: a\n", true},
		{"empty", "automations: []\n", false},
		{"absent", "entities: {}\n", false},
		{"null", "automations:\n", false},
		{"explicit null", "automations: ~\n", false},
		{"mapping", "automations: {name: x}\n", true},
		{"empty mapping", "automations: {}\n", true},
		{"scalar", "automations: x\n", true},
		{"not a mapping", "- a\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tc.input), &doc); err != nil {
				t.Fatal(err)
			}
			if got := (&AutomationsToFlowsMigration{}).Detect(&doc); got != tc.want {
				t.Errorf("Detect = %v, want %v", got, tc.want)
			}
		})
	}
}

// The per-file runner detects the project-level migration but neither
// applies nor counts it, and does not rewrite the file for it.
func TestAutomationsToFlows_RunnerSkips(t *testing.T) {
	fs := storage.NewMemFS()
	const input = "# keep me\nautomations:\n  - name: a\n"
	if err := fs.WriteFile("/s.yaml", []byte(input), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Apply("/s.yaml", FileTypeMetamodel, fs)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if !res.NeedsMigration() || res.MigrationsApplied() != 0 {
		t.Errorf("NeedsMigration=%v applied=%d, want detected but not applied",
			res.NeedsMigration(), res.MigrationsApplied())
	}
	if got, _ := fs.ReadFile("/s.yaml"); string(got) != input {
		t.Errorf("runner rewrote the file:\n%s", got)
	}
	if len(Blocking(res.Detections)) != 1 {
		t.Error("automations-to-flows is not blocking")
	}
	if err := (&AutomationsToFlowsMigration{}).Apply(nil); err != nil {
		t.Errorf("Apply = %v", err)
	}
}

func TestError_RendersProjectLevelAndSyntax(t *testing.T) {
	e := &Error{FilePath: "schema.yaml", Detections: []DetectionResult{
		{Migration: &AutomationsToFlowsMigration{}, Description: "moved"},
		{Migration: &ACLBypassEnumMigration{}, Description: "syntax"},
	}}
	want := "schema.yaml: moved\nschema.yaml uses deprecated syntax:\n  - syntax\n\n" +
		"Run 'rela migrate' to update your project files."
	if e.Error() != want {
		t.Errorf("Error() =\n%s\nwant\n%s", e.Error(), want)
	}
}
