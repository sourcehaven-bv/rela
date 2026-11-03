package metamodel

import (
	"strings"
	"testing"
)

// TestParse_PluralMustBeOneDirectoryName pins that a plural, declared or
// derived from the type name, is usable as one directory name and unique
// across types without regard to case. The plural names the type's
// directory under entities/, so "../relations" would let writes of that
// type land in relations/ (TKT-C2YVX1). Names that only look unusual, such
// as ones with spaces or accents, keep loading.
func TestParse_PluralMustBeOneDirectoryName(t *testing.T) {
	entity := func(name, plural string) string {
		s := "  " + name + ":\n    label: X\n    id_type: manual\n"
		if plural != "" {
			s += "    plural: \"" + plural + "\"\n"
		}
		return s + "    properties:\n      title:\n        type: string\n"
	}
	tests := []struct {
		name     string
		entities string
		wantErr  string
	}{
		{"declared plural", entity("req", "requirements"), ""},
		{"defaulted plural", entity("req", ""), ""},
		{"space and uppercase", entity("req", "To-do lists"), ""},
		{"accent and inner dot", entity("req", "categorieën.v2"), ""},
		{"parent escape", entity("req", "../relations"), `entity "req": plural "../relations" is not a usable directory name`},
		{"dot dot", entity("req", ".."), `entity "req": plural ".." is not a usable directory name`},
		{"hidden", entity("req", ".rela"), `entity "req": plural ".rela" is not a usable directory name`},
		{"nested", entity("req", "a/b"), `entity "req": plural "a/b" is not a usable directory name`},
		{
			"duplicate declared",
			entity("a", "things") + entity("b", "things"),
			`entity "b": plural "things" is also the plural of entity "a"`,
		},
		{
			"declared collides with default",
			entity("item", "") + entity("thing", "items"),
			`entity "thing": plural "items" is also the plural of entity "item"`,
		},
		{
			"duplicate differing in case",
			entity("a", "Things") + entity("b", "things"),
			`entity "b": plural "things" is also the plural of entity "a" (case is ignored)`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte("version: \"1.0\"\nentities:\n" + tc.entities))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Parse error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestParse_DerivedPluralMessage pins the message for a plural the author
// never wrote: it names the type and asks for an explicit plural.
func TestParse_DerivedPluralMessage(t *testing.T) {
	_, err := Parse([]byte("version: \"1.0\"\nentities:\n  .x:\n    label: X\n    id_type: manual\n" +
		"    properties:\n      title:\n        type: string\n"))
	if err == nil || !strings.Contains(err.Error(), `entity ".x": the plural ".xs" derived from the type name`) ||
		!strings.Contains(err.Error(), "set plural: explicitly") {

		t.Fatalf("Parse error = %v", err)
	}
}
