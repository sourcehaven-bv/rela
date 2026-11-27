package migration

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/testutil/logtest"
)

const (
	fileTypeFailing  FileType = "inmemory-test-failing"
	fileTypeStuck    FileType = "inmemory-test-stuck"
	fileTypeAdvisory FileType = "inmemory-test-advisory"
)

func init() {
	Register(&fakeMigration{name: "fake-failing", ft: fileTypeFailing, applyErr: errors.New("boom")})
	Register(&fakeMigration{name: "fake-stuck", ft: fileTypeStuck})
	Register(&fakeMigration{name: "fake-advisory", ft: fileTypeAdvisory, advisory: true})
}

type fakeMigration struct {
	name     string
	ft       FileType
	applyErr error
	advisory bool
}

func (m *fakeMigration) Name() string             { return m.name }
func (m *fakeMigration) Description() string      { return m.name + " description" }
func (m *fakeMigration) FileTypes() []FileType    { return []FileType{m.ft} }
func (m *fakeMigration) Detect(_ *yaml.Node) bool { return true }
func (m *fakeMigration) Apply(_ *yaml.Node) error { return m.applyErr }
func (m *fakeMigration) Advisory() bool           { return m.advisory }

const legacyCardinalitySchema = `relations:
  blocks:
    from: [ticket]
    to: [ticket]
    source_max: 1
`

func TestMigrateBytes_AppliesPendingMigrations(t *testing.T) {
	t.Parallel()

	got, err := MigrateBytes("schema.yaml", []byte(legacyCardinalitySchema), FileTypeMetamodel, nil)
	if err != nil {
		t.Fatalf("MigrateBytes: %v", err)
	}
	if !strings.Contains(string(got.Data), "max_outgoing: 1") || strings.Contains(string(got.Data), "source_max") {
		t.Errorf("migrated data does not carry the rename:\n%s", got.Data)
	}
	if len(got.Applied) != 1 || got.Applied[0].Migration.Name() != "cardinality-rename" {
		t.Errorf("Applied = %v, want [cardinality-rename]", got.Applied)
	}
	if again, _ := MigrateBytes("schema.yaml", got.Data, FileTypeMetamodel, nil); len(again.Applied) != 0 {
		t.Errorf("migrated data still has pending migrations: %v", again.Applied)
	}
}

func TestMigrateBytes_ReturnsInputUnchanged(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		ft   FileType
		data string
	}{
		{name: "nothing pending", ft: FileTypeMetamodel, data: "entities:\n  ticket:\n    id_type: sequential\n"},
		{name: "invalid YAML", ft: FileTypeMetamodel, data: "entities: [unclosed\n"},
		{name: "only advisory pending", ft: fileTypeAdvisory, data: "key: value\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			in := []byte(tc.data)
			got, err := MigrateBytes("f.yaml", in, tc.ft, nil)
			if err != nil {
				t.Fatalf("MigrateBytes: %v", err)
			}
			if !bytes.Equal(got.Data, in) || len(got.Applied) != 0 {
				t.Errorf("got Data=%q Applied=%v, want input unchanged and nothing applied", got.Data, got.Applied)
			}
		})
	}
}

func TestMigrateBytes_Refuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		ft        FileType
		data      string
		wantInErr string
		wantMigEr bool
	}{
		{
			name:      "project-level migration",
			ft:        FileTypeMetamodel,
			data:      "automations:\n  - name: x\n    trigger: on_create\n",
			wantInErr: "schema.yaml",
			wantMigEr: true,
		},
		{
			name:      "privilege rewrite needs review",
			ft:        FileTypeMetamodel,
			data:      bypassTrueYAML,
			wantInErr: "allow_acl_bypass",
			wantMigEr: true,
		},
		{name: "apply error", ft: fileTypeFailing, data: "k: v\n", wantInErr: "fake-failing failed: boom"},
		{name: "pattern survives apply", ft: fileTypeStuck, data: "k: v\n", wantInErr: "fake-stuck", wantMigEr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := MigrateBytes("schema.yaml", []byte(tc.data), tc.ft, nil)
			if err == nil {
				t.Fatalf("MigrateBytes = %q, want an error", got.Data)
			}
			if !strings.Contains(err.Error(), tc.wantInErr) {
				t.Errorf("error %q does not contain %q", err, tc.wantInErr)
			}
			if IsMigrationError(err) != tc.wantMigEr {
				t.Errorf("IsMigrationError = %v, want %v", IsMigrationError(err), tc.wantMigEr)
			}
		})
	}
}

func TestWarnApplied(t *testing.T) { //nolint:paralleltest // swaps the default slog logger
	logged := logtest.Capture(t, slog.LevelWarn)

	WarnApplied("schema.yaml", nil)
	if out := logged(); out != "" {
		t.Fatalf("WarnApplied with nothing applied logged %q", out)
	}

	got, err := MigrateBytes("schema.yaml", []byte(legacyCardinalitySchema), FileTypeMetamodel, nil)
	if err != nil {
		t.Fatalf("MigrateBytes: %v", err)
	}
	WarnApplied("schema.yaml", got.Applied)
	out := logged()
	for _, want := range []string{"level=WARN", "file=schema.yaml", "cardinality-rename", "rela migrate"} {
		if !strings.Contains(out, want) {
			t.Errorf("log %q does not contain %q", out, want)
		}
	}
}

const fileTypeChain FileType = "inmemory-test-chain"

func init() {
	Register(&renameKeyMigration{name: "fake-second", from: "b", to: "c"})
	Register(&renameKeyMigration{name: "fake-first", from: "a", to: "b"})
}

type renameKeyMigration struct {
	name, from, to string
}

func (m *renameKeyMigration) Name() string          { return m.name }
func (m *renameKeyMigration) Description() string   { return m.name + " description" }
func (m *renameKeyMigration) FileTypes() []FileType { return []FileType{fileTypeChain} }

func (m *renameKeyMigration) Detect(doc *yaml.Node) bool {
	return GetMapValue(GetDocumentRoot(doc), m.from) != nil
}

func (m *renameKeyMigration) Apply(doc *yaml.Node) error {
	root := GetDocumentRoot(doc)
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == m.from {
			root.Content[i].Value = m.to
		}
	}
	return nil
}

// A migration whose output another migration rewrites is followed to the
// end, as running `rela migrate` until nothing is pending would.
func TestMigrateBytes_FollowsChainedMigrations(t *testing.T) {
	t.Parallel()

	got, err := MigrateBytes("f.yaml", []byte("a: 1\n"), fileTypeChain, nil)
	if err != nil {
		t.Fatalf("MigrateBytes: %v", err)
	}
	if string(got.Data) != "c: 1\n" {
		t.Errorf("Data = %q, want %q", got.Data, "c: 1\n")
	}
	names := make([]string, len(got.Applied))
	for i, d := range got.Applied {
		names[i] = d.Migration.Name()
	}
	if strings.Join(names, ",") != "fake-first,fake-second" {
		t.Errorf("Applied = %v, want [fake-first fake-second]", names)
	}
}

func TestMigrated_Explain(t *testing.T) {
	t.Parallel()

	base := errors.New("line 3: bad value")
	if got := (Migrated{}).Explain(base); got != base { //nolint:errorlint // identity: the error must come back unwrapped
		t.Errorf("Explain with nothing applied = %v, want the error unchanged", got)
	}
	if got := (Migrated{}).Explain(nil); got != nil {
		t.Errorf("Explain(nil) = %v, want nil", got)
	}
	applied := Migrated{Applied: []DetectionResult{{Migration: &renameKeyMigration{name: "x"}}}}
	got := applied.Explain(base)
	if !errors.Is(got, base) || !strings.Contains(got.Error(), "after in-memory migration") {
		t.Errorf("Explain = %v, want it to wrap the error and name the migrated form", got)
	}
}
