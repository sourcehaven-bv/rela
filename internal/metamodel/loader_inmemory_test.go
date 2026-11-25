package metamodel

import (
	"bytes"
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

func writeLegacySchema(t *testing.T) (fs storage.FS, path string, schema []byte) {
	t.Helper()
	mem := storage.NewMemFS()
	path = "/proj/schema.yaml"
	schema = []byte("entities:\n" + validEntity("ticket", "TKT-") +
		"relations:\n  blocks:\n    label: blocks\n    from: [ticket]\n    to: [ticket]\n    source_max: 1\n")
	if err := mem.MkdirAll("/proj", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := mem.WriteFile(path, schema, 0o644); err != nil {
		t.Fatal(err)
	}
	return mem, path, schema
}

// A schema with deprecated syntax loads through the in-memory migration, as
// it would after `rela migrate`, and the file on disk is left as it was.
func TestFSLoader_MigratesDeprecatedSyntaxInMemory(t *testing.T) {
	fs, path, schema := writeLegacySchema(t)

	m, _, err := NewFSLoader(fs, path).Load(context.Background())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	rel, ok := m.GetRelationDef("blocks")
	if !ok || rel.MaxOutgoing == nil || *rel.MaxOutgoing != 1 {
		t.Errorf("blocks.max_outgoing = %v, want 1 from the migrated source_max", rel.MaxOutgoing)
	}
	onDisk, err := fs.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(onDisk, schema) {
		t.Errorf("schema.yaml changed on disk:\n%s", onDisk)
	}
}

// The strict loader behind `rela validate` still refuses deprecated syntax.
func TestStrictFSLoader_RefusesDeprecatedSyntax(t *testing.T) {
	fs, path, _ := writeLegacySchema(t)

	_, _, err := NewStrictFSLoader(fs, path).Load(context.Background())
	if !migration.IsMigrationError(err) {
		t.Fatalf("err = %v, want a migration error", err)
	}
}

// The strict loader leaves a YAML syntax error to the parser, which reports
// it with a line number.
func TestStrictFSLoader_ReportsInvalidYAMLThroughParser(t *testing.T) {
	fs := storage.NewMemFS()
	const path = "/proj/schema.yaml"
	if err := fs.MkdirAll("/proj", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFile(path, []byte("entities: [unclosed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := NewStrictFSLoader(fs, path).Load(context.Background())
	if err == nil || migration.IsMigrationError(err) {
		t.Fatalf("err = %v, want a parse error", err)
	}
}
