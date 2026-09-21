package datamigration

import (
	"strings"
	"testing"
)

func TestParseFile_ValidRoundTrip(t *testing.T) {
	data := mustFileYAML(t, metaV1(), metaV2(), `  - rename_property: {entity: task, from: status, to: state}
  - map_values:
      entity: task
      property: state
      mapping: {open: todo, wip: doing}
  - convert: {entity: task, property: due, to_type: date, from_format: "01/02/2006"}
`)
	f := mustParse(t, testName("test"), data)
	if len(f.Steps) != 3 {
		t.Fatalf("parsed %d steps, want 3", len(f.Steps))
	}
	if f.FromProjection.Hash() != metaV1().ShapeProjection().Hash() {
		t.Error("from_projection did not survive the round trip")
	}
	if f.ToProjection.Hash() != metaV2().ShapeProjection().Hash() {
		t.Error("to_projection did not survive the round trip")
	}
}

// The point of the change: a migration whose schema shape is identical on both
// sides — a backfill, a de-duplication, a correction of values an old bug
// wrote. Under the previous hash-edge model this was refused at parse time
// because `from` and `to` were the same hash (TKT-XCJ0Y2).
func TestParseFile_DataOnlyMigration(t *testing.T) {
	v1 := metaV1()
	data := mustFileYAML(t, v1, v1,
		"  - set_default: {entity: task, property: status, value: open}\n")

	f, err := ParseFile(testName("backfill-status"), data)
	if err != nil {
		t.Fatalf("a data-only migration must parse: %v", err)
	}
	if len(f.Steps) != 1 {
		t.Fatalf("parsed %d steps, want 1", len(f.Steps))
	}
	if f.FromProjection.Hash() != f.ToProjection.Hash() {
		t.Error("a data-only migration spans no shape change; the projections should match")
	}
}

// A migration file is addressed by name, and the name becomes an applied-list
// entry that is later compared against directory entries — so it must satisfy
// the allowlist here rather than at some later use.
func TestParseFile_RejectsUnsafeName(t *testing.T) {
	data := mustFileYAML(t, metaV1(), metaV1(), "  []\n")
	for _, name := range []string{
		"../../escape.yaml",
		"0001-legacy.yaml",
		"20260919143022-Uppercase.yaml",
		"no-timestamp.yaml",
	} {
		if _, err := ParseFile(name, data); err == nil {
			t.Errorf("ParseFile(%q) accepted an invalid migration name", name)
		}
	}
}

func TestParseFile_Rejections(t *testing.T) {
	v1, v2 := metaV1(), metaV2()
	valid := string(mustFileYAML(t, v1, v2, "  - rename_property: {entity: task, from: status, to: state}\n"))

	tests := []struct {
		name    string
		mutate  func(string) string
		wantErr string
	}{
		{
			name:    "unknown step kind",
			mutate:  func(s string) string { return strings.Replace(s, "rename_property", "rename_prop", 1) },
			wantErr: "unknown step kind",
		},
		{
			name:    "unknown field in step",
			mutate:  func(s string) string { return strings.Replace(s, "from: status", "form: status", 1) },
			wantErr: "field form not found",
		},
		{
			name: "step targeting unknown property",
			mutate: func(s string) string {
				return strings.Replace(s, "from: status", "from: nonexistent", 1)
			},
			wantErr: "not in the from-schema",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFile(testName("test"), []byte(tc.mutate(valid)))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestParseFile_DropOnLiveTypeRejected(t *testing.T) {
	// drop_entities may only target types the NEW schema no longer knows.
	data := mustFileYAML(t, metaV1(), metaV2(), "  - drop_entities: {type: person}\n")
	_, err := ParseFile(testName("test"), data)
	if err == nil || !strings.Contains(err.Error(), "still exists in the to-schema") {
		t.Fatalf("error = %v, want still-exists rejection", err)
	}
}

func TestParseFile_MissingProjection(t *testing.T) {
	data := "description: no projections\nsteps: []\n"
	_, err := ParseFile(testName("test"), []byte(data))
	if err == nil || !strings.Contains(err.Error(), "must embed the projection") {
		t.Fatalf("error = %v, want missing-projection rejection", err)
	}
}
