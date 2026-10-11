package metamodel

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/propmatch"
)

const refSchemaHead = `
version: "1.0"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    properties:
      title:
        type: string
`

func TestParse_ExternalRef(t *testing.T) {
	tests := []struct {
		name    string
		extra   string // appended under ticket.properties
		tail    string // appended after the entity block
		wantErr string
	}{
		{name: "valid", extra: "      basecamp:\n        type: external_ref\n        system: basecamp\n        sync: true\n"},
		{name: "valid without sync", extra: "      jira:\n        type: external_ref\n        system: jira\n"},
		{name: "missing system", extra: "      basecamp:\n        type: external_ref\n", wantErr: `"system" is required`},
		{name: "bad system", extra: "      basecamp:\n        type: external_ref\n        system: Base Camp\n", wantErr: "must match"},
		{name: "system on string", extra: "      basecamp:\n        type: string\n        system: basecamp\n", wantErr: `sets "system"`},
		{name: "sync on string", extra: "      basecamp:\n        type: string\n        sync: true\n", wantErr: `sets "sync"`},
		{name: "list", extra: "      basecamp:\n        type: external_ref\n        system: basecamp\n        list: true\n", wantErr: `"list" cannot be combined`},
		{name: "unique", extra: "      basecamp:\n        type: external_ref\n        system: basecamp\n        unique: true\n", wantErr: `"unique" cannot be combined`},
		{name: "default", extra: "      basecamp:\n        type: external_ref\n        system: basecamp\n        default: x\n", wantErr: `"default" cannot be combined`},
		{name: "computed", extra: "      basecamp:\n        type: external_ref\n        system: basecamp\n        computed: entity.title\n", wantErr: `"computed" cannot be combined`},
		{name: "values", extra: "      basecamp:\n        type: external_ref\n        system: basecamp\n        values: [a]\n", wantErr: `"values" cannot be combined`},
		{name: "format", extra: "      basecamp:\n        type: external_ref\n        system: basecamp\n        format: x\n", wantErr: `"format" cannot be combined`},
		{name: "required", extra: "      basecamp:\n        type: external_ref\n        system: basecamp\n        required: true\n", wantErr: `"required" cannot be combined`},
		{
			name:    "two props one system",
			extra:   "      a:\n        type: external_ref\n        system: basecamp\n      b:\n        type: external_ref\n        system: basecamp\n",
			wantErr: "a type may declare each system once",
		},
		{
			name:    "display property",
			extra:   "      basecamp:\n        type: external_ref\n        system: basecamp\n",
			tail:    "",
			wantErr: "",
		},
		{
			name:    "property trigger",
			extra:   "      basecamp:\n        type: external_ref\n        system: basecamp\n",
			tail:    "automations:\n  - name: on-ref\n    on:\n      entity: ticket\n      property: basecamp\n    do:\n      - set: \"title=x\"\n",
			wantErr: "cannot trigger an automation",
		},
		{
			name:    "automation set",
			extra:   "      basecamp:\n        type: external_ref\n        system: basecamp\n",
			tail:    "automations:\n  - name: set-ref\n    on:\n      entity: ticket\n      created: true\n    do:\n      - set: basecamp\n        value: \"x\"\n",
			wantErr: "`set` \"basecamp\" is an external ref",
		},
		{
			name:    "automation set on every type",
			extra:   "      basecamp:\n        type: external_ref\n        system: basecamp\n",
			tail:    "automations:\n  - name: set-ref\n    on:\n      created: true\n    do:\n      - set: basecamp\n        value: \"x\"\n",
			wantErr: "only a script may write a ref",
		},
		{
			name:  "automation create_entity property",
			extra: "      basecamp:\n        type: external_ref\n        system: basecamp\n",
			tail: "automations:\n  - name: spawn\n    on:\n      entity: ticket\n      created: true\n    do:\n" +
				"      - create_entity:\n          type: ticket\n          properties:\n            title: x\n            basecamp: \"{{new.title}}\"\n",
			wantErr: "`create_entity` property \"basecamp\" is an external ref",
		},
		{
			name:    "relation property",
			tail:    "relations:\n  links:\n    label: Links\n    from: [ticket]\n    to: [ticket]\n    properties:\n      ref:\n        type: external_ref\n        system: basecamp\n",
			wantErr: "not supported on relation properties",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse([]byte(refSchemaHead + tc.extra + tc.tail))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

func TestParse_ExternalRefDisplayProperty(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    display_property: basecamp
    properties:
      basecamp:
        type: external_ref
        system: basecamp
`
	_, err := Parse([]byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "display_property") {
		t.Fatalf("error = %v, want a display_property error", err)
	}
}

func TestParse_ExternalRefSyncOnFacedType(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  page:
    label: Page
    id_prefix: "PG-"
    faces:
      draft: {}
      published: {}
    properties:
      title:
        type: string
      basecamp:
        type: external_ref
        system: basecamp
        sync: true
`
	_, err := Parse([]byte(yaml))
	if err == nil || !strings.Contains(err.Error(), "type with faces") {
		t.Fatalf("error = %v, want a faces error", err)
	}
}

func TestParseExternalRef(t *testing.T) {
	long := strings.Repeat("x", ExternalRefIDMaxBytes+1)
	tests := []struct {
		name    string
		in      any
		want    ExternalRefValue
		wantErr string
	}{
		{name: "id only", in: map[string]any{"id": "42"}, want: ExternalRefValue{ID: "42"}},
		{name: "id and url", in: map[string]any{"id": "42", "url": "https://x.test/42"}, want: ExternalRefValue{ID: "42", URL: "https://x.test/42"}},
		{name: "string map", in: map[string]string{"id": "a"}, want: ExternalRefValue{ID: "a"}},
		{name: "empty url", in: map[string]any{"id": "a", "url": ""}, want: ExternalRefValue{ID: "a"}},
		{name: "number id", in: map[string]any{"id": int64(42)}, wantErr: "quote numeric ids"},
		{name: "float id", in: map[string]any{"id": 42.0}, wantErr: "quote numeric ids"},
		{name: "missing id", in: map[string]any{"url": "https://x.test"}, wantErr: `"id" is required`},
		{name: "empty id", in: map[string]any{"id": ""}, wantErr: "must not be empty"},
		{name: "long id", in: map[string]any{"id": long}, wantErr: "longer than"},
		{name: "control char", in: map[string]any{"id": "a\nb"}, wantErr: "control"},
		{name: "right-to-left override", in: map[string]any{"id": "a\u202Eb"}, wantErr: "format"},
		{name: "zero-width space", in: map[string]any{"id": "a\u200Bb"}, wantErr: "format"},
		{name: "byte order mark", in: map[string]any{"id": "\uFEFFa"}, wantErr: "format"},
		{name: "non-breaking space", in: map[string]any{"id": "a\u00A0b"}, wantErr: "format"},
		{name: "unicode letters", in: map[string]any{"id": "taak-é-42"}, want: ExternalRefValue{ID: "taak-é-42"}},
		{name: "unknown key", in: map[string]any{"id": "a", "rev": "1"}, wantErr: `unknown key "rev"`},
		{name: "javascript url", in: map[string]any{"id": "a", "url": "javascript:alert(1)"}, wantErr: "http or https"},
		{name: "no host", in: map[string]any{"id": "a", "url": "https:///x"}, wantErr: "host"},
		{name: "url not string", in: map[string]any{"id": "a", "url": 3}, wantErr: `"url" must be a string`},
		{name: "list", in: []any{map[string]any{"id": "a"}}, wantErr: "must be an object"},
		{name: "string", in: "a", wantErr: "must be an object"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseExternalRef(tc.in)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) || !errors.Is(err, ErrInvalidExternalRef) {
					t.Fatalf("error = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestValidateEntity_ExternalRefIsHard(t *testing.T) {
	m, err := Parse([]byte(refSchemaHead + "      basecamp:\n        type: external_ref\n        system: basecamp\n"))
	if err != nil {
		t.Fatal(err)
	}
	errs := m.ValidateEntity("TKT-1", "ticket", map[string]any{"basecamp": map[string]any{"id": 7}})
	if len(errs) != 1 || errs[0].Type != ValidationErrorExternalRef || errs[0].IsSoft() {
		t.Fatalf("errs = %v, want one hard external-ref error", errs)
	}
	// D10: `{}` is not "no value"; it is a ref without an id, refused at
	// write so it never stores.
	errs = m.ValidateEntity("TKT-1", "ticket", map[string]any{"basecamp": map[string]any{}})
	if len(errs) != 1 || errs[0].Type != ValidationErrorExternalRef {
		t.Fatalf("errs = %v, want the empty map refused as a ref without an id", errs)
	}
}

// D10: one definition of empty, shared with propmatch (which backs the
// naive store and the filter DSL; the SQL backends are held to it by the
// storetest differential).
func TestIsEmptyValue(t *testing.T) {
	for _, v := range []any{nil, "", []any{}, []string{}} {
		if !IsEmptyValue(v) || !propmatch.IsEmpty(v) {
			t.Errorf("%#v: IsEmptyValue=%v propmatch.IsEmpty=%v, want both true", v, IsEmptyValue(v), propmatch.IsEmpty(v))
		}
	}
	for _, v := range []any{"a", 0, false, map[string]any{}, map[string]any{"id": "a"}} {
		if IsEmptyValue(v) || propmatch.IsEmpty(v) {
			t.Errorf("%#v: IsEmptyValue=%v propmatch.IsEmpty=%v, want both false", v, IsEmptyValue(v), propmatch.IsEmpty(v))
		}
	}
}

func TestShapeProjection_ExternalRefSystem(t *testing.T) {
	load := func(system string) *Metamodel {
		t.Helper()
		m, err := Parse([]byte(refSchemaHead + "      ref:\n        type: external_ref\n        system: " + system + "\n"))
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	a, b := load("basecamp").ShapeProjection(), load("jira").ShapeProjection()
	if a.Hash() == b.Hash() {
		t.Fatal("a system change must move the shape hash")
	}
	if r := CompareShapes(a, b); r.Tier() != TierMigration {
		t.Fatalf("verdict = %v, want needs-migration", r.Tier())
	}
	if load("basecamp").RenderProjection().Hash() != load("jira").RenderProjection().Hash() {
		t.Fatal("system must not join the render projection")
	}
}

func TestSyncRefProps(t *testing.T) {
	m, err := Parse([]byte(refSchemaHead +
		"      basecamp:\n        type: external_ref\n        system: basecamp\n        sync: true\n" +
		"      jira:\n        type: external_ref\n        system: jira\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := SyncRefProps(m); len(got) != 1 || got[0] != "ticket.basecamp" {
		t.Fatalf("SyncRefProps = %v", got)
	}
	if got := ExternalRefProps(m, "jira"); len(got) != 1 || got[0].Property != "jira" {
		t.Fatalf("ExternalRefProps = %v", got)
	}
}
