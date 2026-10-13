package predicate

import "testing"

func usesWholeEnv(t *testing.T) *Env {
	t.Helper()
	env := NewEnv()
	entity := RecordType{
		"status": StringType,
		"tags":   ListType{Elem: StringType},
		"meta":   RecordType{"kind": StringType},
	}
	mustDeclareVar(t, env, "entity", entity)
	mustDeclareVar(t, env, "current_user", RecordType{"id": StringType})
	mustDeclareVar(t, env, "name", StringType)
	mustDeclareFunc(t, env, "len", FuncSig{Params: []Type{ListType{Elem: StringType}}, Return: NumberType})
	mustDeclareFunc(t, env, "inspect", FuncSig{Params: []Type{RecordType{}}, Return: BoolType})
	return env
}

func mustDeclareVar(t *testing.T, env *Env, name string, typ Type) {
	t.Helper()
	if err := env.DeclareVar(name, typ); err != nil {
		t.Fatal(err)
	}
}

func mustDeclareFunc(t *testing.T, env *Env, name string, sig FuncSig) {
	t.Helper()
	if err := env.DeclareFunc(name, sig); err != nil {
		t.Fatal(err)
	}
}

// TestProgram_UsesWhole pins one case per expression form in which a
// variable can appear. Only attribute access is a field-limited use; every
// other form hands the whole record on.
func TestProgram_UsesWhole(t *testing.T) {
	tests := []struct {
		name  string
		src   string
		whole bool
		attrs []string
	}{
		{"attribute comparison", "entity.status == 'open'", false, []string{"status"}},
		{"bracket attribute", "entity['status'] == 'open'", false, []string{"status"}},
		{"attribute passed to a function", "len(entity.tags) > 0", false, []string{"tags"}},
		{"nested attribute", "entity.meta.kind == 'x'", false, []string{"meta"}},
		{"attribute under not", "not (entity.status == 'open')", false, []string{"status"}},
		{"attribute selection", "(entity.status or 'none') == 'open'", false, []string{"status"}},
		{"record passed to a function", "inspect(entity)", true, nil},
		{"traversal subject", "related(entity, 'owns')", true, nil},
		{"traversal subject with a constraint", "related(entity, 'owns', { owner = current_user.id })", true, nil},
		{"whole use beside an attribute", "entity.status == 'open' and inspect(entity)", true, []string{"status"}},
		{"another variable only", "current_user.id == 'u'", false, nil},
		{"traversal over another variable", "related(current_user, 'owns')", false, nil},
	}
	// A scalar variable compared to nil is a whole use of that variable.
	prog, err := Compile(usesWholeEnv(t), "name == nil")
	if err != nil {
		t.Fatalf("compile name == nil: %v", err)
	}
	if !prog.UsesWhole("name") || prog.UsesWhole("entity") {
		t.Errorf("name == nil: UsesWhole(name) = %v, UsesWhole(entity) = %v; want true, false",
			prog.UsesWhole("name"), prog.UsesWhole("entity"))
	}
	// A record cannot be compared, so `entity == nil` never reaches UsesWhole.
	if _, err := Compile(usesWholeEnv(t), "entity == nil"); err == nil {
		t.Error("entity == nil compiled; UsesWhole's godoc says a record cannot be compared")
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := Compile(usesWholeEnv(t), tc.src)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			if got := prog.UsesWhole("entity"); got != tc.whole {
				t.Errorf("UsesWhole(entity) = %v, want %v", got, tc.whole)
			}
			got := prog.Attributes("entity")
			if len(got) != len(tc.attrs) {
				t.Fatalf("Attributes(entity) = %v, want %v", got, tc.attrs)
			}
			for i := range got {
				if got[i] != tc.attrs[i] {
					t.Fatalf("Attributes(entity) = %v, want %v", got, tc.attrs)
				}
			}
			if (tc.whole || len(tc.attrs) > 0) && !prog.References("entity") {
				t.Error("References(entity) = false for a program that reads it")
			}
		})
	}
}

// TestProgram_UsesWhole_ValueProfile covers the forms only a value profile
// admits: arithmetic and concatenation over an attribute stay field-limited.
func TestProgram_UsesWhole_ValueProfile(t *testing.T) {
	tests := []struct {
		name    string
		src     string
		profile Profile
	}{
		{"arithmetic", "len(entity.tags) + 1", Profile{Expected: NumberType, AllowArithmetic: true}},
		{"unary minus", "-len(entity.tags)", Profile{Expected: NumberType, AllowArithmetic: true}},
		{"concatenation", "entity.status .. 'x'", Profile{Expected: StringType, AllowConcatenation: true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			prog, err := CompileValue(usesWholeEnv(t), tc.src, tc.profile)
			if err != nil {
				t.Fatalf("compile %q: %v", tc.src, err)
			}
			if prog.UsesWhole("entity") {
				t.Error("UsesWhole(entity) = true for attribute-only access")
			}
			if len(prog.Attributes("entity")) != 1 {
				t.Errorf("Attributes(entity) = %v, want one field", prog.Attributes("entity"))
			}
		})
	}
}
