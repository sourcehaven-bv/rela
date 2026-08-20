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
	f := mustParse(t, "0001-test.yaml", data)
	if len(f.Steps) != 3 {
		t.Fatalf("parsed %d steps, want 3", len(f.Steps))
	}
	if f.From != metaV1().ShapeProjection().Hash() || f.To != metaV2().ShapeProjection().Hash() {
		t.Fatalf("hashes not carried through")
	}
	if f.FromProjection.Hash() != f.From {
		t.Fatalf("embedded projection does not match hash after round-trip")
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
			name:    "bad from hash",
			mutate:  func(s string) string { return "from: nothex\n" + s[strings.Index(s, "to:"):] },
			wantErr: "`from` is not a shape hash",
		},
		{
			name: "projection hash mismatch",
			mutate: func(s string) string {
				// Corrupt the declared `to` hash: still 64 hex chars, but no
				// longer the embedded projection's hash.
				idx := strings.Index(s, "to: ")
				return s[:idx+4] + "0000" + s[idx+8:]
			},
			wantErr: "to_projection hashes to",
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
			_, err := ParseFile("0001-test.yaml", []byte(tc.mutate(valid)))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want containing %q", err, tc.wantErr)
			}
		})
	}
}

func TestParseFile_DropOnLiveTypeRejected(t *testing.T) {
	// drop_entities may only target types the NEW schema no longer knows.
	data := mustFileYAML(t, metaV1(), metaV2(), "  - drop_entities: {type: person}\n")
	_, err := ParseFile("0001-test.yaml", data)
	if err == nil || !strings.Contains(err.Error(), "still exists in the to-schema") {
		t.Fatalf("error = %v, want still-exists rejection", err)
	}
}

func TestParseFile_MissingProjection(t *testing.T) {
	v1, v2 := metaV1(), metaV2()
	data := "from: " + v1.ShapeProjection().Hash() + "\nto: " + v2.ShapeProjection().Hash() + "\nsteps: []\n"
	_, err := ParseFile("0001-test.yaml", []byte(data))
	if err == nil || !strings.Contains(err.Error(), "must embed the projection") {
		t.Fatalf("error = %v, want missing-projection rejection", err)
	}
}
