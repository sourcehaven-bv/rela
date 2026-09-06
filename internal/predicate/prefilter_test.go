package predicate

import "testing"

// prefilterEnv is the request-scoped shape: an entity record with mixed
// property types plus a constant current_user record.
func prefilterEnv(t *testing.T) *Env {
	t.Helper()
	env := NewEnv()
	if err := env.DeclareVar("entity", RecordType{
		"assignee": StringType,
		"reporter": StringType,
		"status":   StringType,
		"count":    IntType,
		"done":     BoolType,
		"tags":     ListType{Elem: StringType},
	}); err != nil {
		t.Fatalf("declare entity: %v", err)
	}
	if err := env.DeclareVar("current_user", RecordType{
		"id":   StringType,
		"tool": StringType,
	}); err != nil {
		t.Fatalf("declare current_user: %v", err)
	}
	if err := env.DeclareFunc("is_current_user", FuncSig{
		Params: []Type{StringType}, Return: BoolType, SQLPortable: true,
	}); err != nil {
		t.Fatalf("declare is_current_user: %v", err)
	}
	return env
}

func TestConstEqualities(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []ConstEquality
	}{
		{
			name: "equality against the constant record is pushable",
			src:  "entity.assignee == current_user.id",
			want: []ConstEquality{{Attribute: "assignee", FromVar: "id"}},
		},
		{
			name: "reversed operand order is the same constraint",
			src:  "current_user.id == entity.assignee",
			want: []ConstEquality{{Attribute: "assignee", FromVar: "id"}},
		},
		{
			name: "string literal equality is pushable",
			src:  "entity.status == 'open'",
			want: []ConstEquality{{Attribute: "status", Value: "open"}},
		},
		{
			name: "an AND chain yields every conjunct",
			src:  "entity.status == 'open' and entity.assignee == current_user.id",
			want: []ConstEquality{
				{Attribute: "assignee", FromVar: "id"},
				{Attribute: "status", Value: "open"},
			},
		},
		{
			name: "nested AND is still a top-level chain",
			src:  "(entity.status == 'open' and entity.reporter == 'x') and entity.assignee == current_user.id",
			want: []ConstEquality{
				{Attribute: "assignee", FromVar: "id"},
				{Attribute: "reporter", Value: "x"},
				{Attribute: "status", Value: "open"},
			},
		},

		// --- soundness restrictions: each of these MUST push nothing ---
		{
			name: "OR pushes nothing: either branch may be false in a matching row",
			src:  "entity.assignee == current_user.id or entity.reporter == current_user.id",
			want: nil,
		},
		{
			name: "an AND under an OR pushes nothing",
			src:  "entity.status == 'open' or (entity.assignee == current_user.id and entity.reporter == 'x')",
			want: nil,
		},
		{
			name: "NOT inverts the sense, so pushes nothing",
			src:  "not (entity.assignee == current_user.id)",
			want: nil,
		},
		{
			name: "inequality pushes nothing",
			src:  "entity.assignee ~= current_user.id",
			want: nil,
		},
		{
			name: "ordered comparison pushes nothing",
			src:  "entity.status > 'open'",
			want: nil,
		},
		{
			name: "a host-func call is not an equality",
			src:  "is_current_user(entity.assignee)",
			want: nil,
		},
		{
			name: "field-to-field equality varies per row",
			src:  "entity.assignee == entity.reporter",
			want: nil,
		},
		{
			name: "a typed (int) equality is not pushed: string-form comparison could disagree",
			src:  "entity.count == 3",
			want: nil,
		},
		{
			name: "a bool equality is not pushed either",
			src:  "entity.done == true",
			want: nil,
		},
		{
			name: "the constant record's own fields are not entity attributes",
			src:  "current_user.id == 'PERS-JV'",
			want: nil,
		},
		{
			name: "a mixed AND still pushes the sound conjunct only",
			src:  "entity.assignee == current_user.id and entity.count == 3",
			want: []ConstEquality{{Attribute: "assignee", FromVar: "id"}},
		},
		{
			name: "an OR beside a pushable AND conjunct keeps the conjunct",
			src:  "entity.status == 'open' and (entity.assignee == current_user.id or entity.reporter == 'x')",
			want: []ConstEquality{{Attribute: "status", Value: "open"}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := Compile(prefilterEnv(t), tc.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			got := prog.ConstEqualities("entity", "current_user")
			if len(got) != len(tc.want) {
				t.Fatalf("got %d equalities %+v, want %d %+v", len(got), got, len(tc.want), tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("equality %d: got %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestConstEqualities_WithoutConstVar proves the constant record is
// opt-in: a caller that names no constVar gets literals only, never a
// field whose value it has no way to supply.
func TestConstEqualities_WithoutConstVar(t *testing.T) {
	prog, err := Compile(prefilterEnv(t),
		"entity.status == 'open' and entity.assignee == current_user.id")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := prog.ConstEqualities("entity", "")
	want := []ConstEquality{{Attribute: "status", Value: "open"}}
	if len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

// TestConstEqualities_NilAndEmpty covers the degenerate inputs rather
// than leaving them to a panic at a call site.
func TestConstEqualities_NilAndEmpty(t *testing.T) {
	var nilProg *Program
	if got := nilProg.ConstEqualities("entity", "current_user"); got != nil {
		t.Fatalf("nil program: got %+v, want nil", got)
	}
	prog, err := Compile(prefilterEnv(t), "entity.status == 'open'")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if got := prog.ConstEqualities("", "current_user"); got != nil {
		t.Fatalf("empty recordVar: got %+v, want nil", got)
	}
	if got := prog.ConstEqualities("nosuchvar", "current_user"); got != nil {
		t.Fatalf("unknown recordVar: got %+v, want nil", got)
	}
}

// TestConstEqualities_DuplicateAttributeReportedOnce pins the documented
// contract that the result is a SUBSET of the program's constraints: the
// Go pass remains authoritative for the one that is dropped.
func TestConstEqualities_DuplicateAttributeReportedOnce(t *testing.T) {
	prog, err := Compile(prefilterEnv(t),
		"entity.status == 'open' and entity.status == 'closed'")
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := prog.ConstEqualities("entity", "current_user")
	if len(got) != 1 || got[0].Attribute != "status" {
		t.Fatalf("got %+v, want a single status equality", got)
	}
}
