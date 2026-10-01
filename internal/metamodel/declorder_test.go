package metamodel

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const twoWorlds = `worlds:
  published:
    select: published
    otherwise: exclude
  editorial:
    select: [review, published]
    otherwise: default
`

// TestDeclOrder_FromYAML pins that faces and worlds come back in the order
// the YAML declares them, not sorted: policy declares draft, review,
// published, and the worlds are published then editorial.
func TestDeclOrder_FromYAML(t *testing.T) {
	t.Parallel()
	m, err := Parse([]byte(worldsSchema(twoWorlds)))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got, want := FaceOrderOf(m, "policy"), []string{"draft", "review", "published"}; !slices.Equal(got, want) {
		t.Errorf("FaceOrderOf(policy) = %v, want %v", got, want)
	}
	if got := FaceOrderOf(m, "ticket"); got != nil {
		t.Errorf("FaceOrderOf(ticket) = %v, want nil for a faceless type", got)
	}
	if got, want := WorldOrderOf(m), []string{"published", "editorial"}; !slices.Equal(got, want) {
		t.Errorf("WorldOrderOf = %v, want %v", got, want)
	}

	// The accessors return copies.
	FaceOrderOf(m, "policy")[0] = "mutated"
	WorldOrderOf(m)[0] = "mutated"
	if FaceOrderOf(m, "policy")[0] != "draft" || WorldOrderOf(m)[0] != "published" {
		t.Error("mutating a returned order changed the metamodel")
	}
}

// TestDeclOrder_GoBuiltFallsBackToSortedNames pins the fallback for a
// metamodel built in Go, which records no order.
func TestDeclOrder_GoBuiltFallsBackToSortedNames(t *testing.T) {
	t.Parallel()
	built := &Metamodel{Entities: map[string]EntityDef{
		"policy": {Faces: map[string]FaceDef{"review": {}, "draft": {}, "published": {}}},
	}}
	if got, want := FaceOrderOf(built, "policy"), []string{"draft", "published", "review"}; !slices.Equal(got, want) {
		t.Errorf("FaceOrderOf = %v, want %v", got, want)
	}
	built.Entities["policy"] = EntityDef{Aliases: []string{"pol"}, Faces: built.Entities["policy"].Faces}
	built.InitAliases()
	if got, want := FaceOrderOf(built, "pol"), []string{"draft", "published", "review"}; !slices.Equal(got, want) {
		t.Errorf("FaceOrderOf(alias) = %v, want %v", got, want)
	}
	if got := FaceOrderOf(built, "nosuch"); got != nil {
		t.Errorf("FaceOrderOf(unknown type) = %v, want nil", got)
	}
	if got := FaceOrderOf(nil, "policy"); got != nil {
		t.Errorf("FaceOrderOf(nil metamodel) = %v, want nil", got)
	}
	m := &Metamodel{Worlds: map[string]WorldDef{"b": {}, "a": {}}}
	if got, want := WorldOrderOf(m), []string{"a", "b"}; !slices.Equal(got, want) {
		t.Errorf("WorldOrderOf = %v, want %v", got, want)
	}
	if WorldOrderOf(nil) != nil || WorldOrderOf(&Metamodel{}) != nil {
		t.Error("WorldOrderOf of a nil or worldless metamodel is not nil")
	}
}

// TestDeclOrder_FromIncludedFile pins design A7: a faced type declared in an
// included file keeps its YAML order, and a nested include does too.
func TestDeclOrder_FromIncludedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "metamodel.yaml"), `
version: "1.0"
includes:
  - pages.yaml
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string}
`+twoWorlds)
	createFile(t, filepath.Join(dir, "pages.yaml"), `
includes:
  - policies.yaml
entities:
  page:
    label: Page
    id_prefix: PAGE
    properties:
      title: {type: string}
    faces:
      published: {}
      draft: {}
`)
	createFile(t, filepath.Join(dir, "policies.yaml"), `
entities:
  policy:
    label: Policy
    id_prefix: POL
    properties:
      title: {type: string}
    faces:
      review: {}
      draft: {}
      published: {}
`)
	m, _, err := Load(filepath.Join(dir, "metamodel.yaml"), testMetaFS)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := FaceOrderOf(m, "page"), []string{"published", "draft"}; !slices.Equal(got, want) {
		t.Errorf("FaceOrderOf(page) = %v, want %v", got, want)
	}
	if got, want := FaceOrderOf(m, "policy"), []string{"review", "draft", "published"}; !slices.Equal(got, want) {
		t.Errorf("FaceOrderOf(policy) = %v, want %v", got, want)
	}
}

// TestValidateDeclOrder pins the load-time assertion: a loaded metamodel's
// faced types and worlds carry an order, and it names exactly their keys.
func TestValidateDeclOrder(t *testing.T) {
	t.Parallel()
	faces := map[string]FaceDef{"draft": {}, "published": {}}
	worlds := map[string]WorldDef{"a": {}, "b": {}}
	cases := []struct {
		name string
		m    *Metamodel
		want string
	}{
		{name: "recorded orders pass", m: &Metamodel{
			Entities:   map[string]EntityDef{"page": {Faces: faces, faceOrder: []string{"published", "draft"}}},
			Worlds:     worlds,
			worldOrder: []string{"b", "a"},
		}},
		{name: "faceless type needs no order", m: &Metamodel{
			Entities: map[string]EntityDef{"ticket": {}},
		}},
		{name: "faced type without order", m: &Metamodel{
			Entities: map[string]EntityDef{"page": {Faces: faces}},
		}, want: `entity "page": internal: the declaration order of its faces was not recorded`},
		{name: "face order missing a face", m: &Metamodel{
			Entities: map[string]EntityDef{"page": {Faces: faces, faceOrder: []string{"draft"}}},
		}, want: `entity "page": internal: recorded face order [draft] does not match`},
		{name: "face order with a duplicate", m: &Metamodel{
			Entities: map[string]EntityDef{"page": {Faces: faces, faceOrder: []string{"draft", "draft"}}},
		}, want: "does not match its declared faces"},
		{name: "worlds without order", m: &Metamodel{Worlds: worlds},
			want: "worlds: internal: the declaration order of the worlds was not recorded"},
		{name: "world order naming an undeclared world", m: &Metamodel{Worlds: worlds, worldOrder: []string{"a", "c"}},
			want: "does not match the declared worlds"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			errs := validateDeclOrder(tc.m)
			if tc.want == "" {
				if len(errs) != 0 {
					t.Fatalf("validateDeclOrder = %v, want none", errs)
				}
				return
			}
			if len(errs) != 1 || !strings.Contains(errs[0], tc.want) {
				t.Fatalf("validateDeclOrder = %v, want one error containing %q", errs, tc.want)
			}
		})
	}
}

// TestDefaultWorldKey pins the top-level `default_world:` key: accepted as a
// key, and validated against the declared worlds (TKT-7IZHP0 §21 D3).
func TestDefaultWorldKey(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		worlds  string
		value   string
		wantErr string
	}{
		{name: "unset with worlds", worlds: twoWorlds},
		{name: "unset without worlds"},
		{name: "a declared world", worlds: twoWorlds, value: "editorial"},
		{name: "the generated world without worlds", value: "default"},
		{name: "an undeclared world", worlds: twoWorlds, value: "publsihed",
			wantErr: `default_world: "publsihed" is not a declared world (declared, in order: [published editorial])`},
		{name: "the generated world beside declared worlds", worlds: twoWorlds, value: "default",
			wantErr: `default_world: "default" is not a declared world`},
		{name: "a world when none is declared", value: "published",
			wantErr: `default_world: "published" is not a world; no worlds are declared`},
		{name: "case matters", worlds: twoWorlds, value: "Published",
			wantErr: `default_world: "Published" is not a declared world`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := worldsSchema(tc.worlds)
			if tc.value != "" {
				src += "default_world: " + tc.value + "\n"
			}
			m, err := Parse([]byte(src))
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				if m.DefaultWorld != tc.value {
					t.Errorf("DefaultWorld = %q, want %q", m.DefaultWorld, tc.value)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("Parse err = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}

// TestDefaultWorldKey_RootOnly pins that an included file may not set
// default_world: it names a world, and worlds are root-only.
func TestDefaultWorldKey_RootOnly(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "metamodel.yaml"), `
version: "1.0"
includes:
  - part.yaml
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string}
`)
	createFile(t, filepath.Join(dir, "part.yaml"), `
default_world: default
entities:
  note:
    label: Note
    id_prefix: NOTE
    properties:
      title: {type: string}
`)
	_, _, err := Load(filepath.Join(dir, "metamodel.yaml"), testMetaFS)
	var rootErr *IncludeHasRootFieldError
	if !errors.As(err, &rootErr) || rootErr.Field != "default_world" {
		t.Fatalf("Load err = %v, want an IncludeHasRootFieldError for default_world", err)
	}
}

// anchoredSchema declares faces through YAML anchors, aliases and merge keys,
// which the decoder resolves and the order extraction must resolve the same
// way.
const anchoredSchema = `version: "1.0"
entities:
  page:
    label: Page
    id_prefix: PAGE
    properties:
      title: {type: string}
    faces: &pf
      published: {}
      draft: {}
  note:
    label: Note
    id_prefix: NOTE
    properties:
      title: {type: string}
    faces: *pf
  memo:
    label: Memo
    id_prefix: MEMO
    properties:
      title: {type: string}
    faces:
      <<: *pf
      archived: {}
  brief: &brief
    label: Brief
    id_prefix: BRF
    properties:
      title: {type: string}
    faces:
      review: {}
      draft: {}
  letter:
    <<: *brief
    label: Letter
    id_prefix: LTR
  digest:
    label: Digest
    id_prefix: DIG
    properties:
      title: {type: string}
    faces:
      <<: [*pf, {review: {}}]
      draft: {}
`

// TestDeclOrder_AnchorsAliasesAndMergeKeys pins that anchors, aliases and
// merge keys load, and that each order names exactly the decoded faces, in
// document position.
func TestDeclOrder_AnchorsAliasesAndMergeKeys(t *testing.T) {
	t.Parallel()
	m, err := Parse([]byte(anchoredSchema))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	for typ, want := range map[string][]string{
		"page":   {"published", "draft"},
		"note":   {"published", "draft"},
		"memo":   {"published", "draft", "archived"},
		"brief":  {"review", "draft"},
		"letter": {"review", "draft"},
		"digest": {"published", "draft", "review"},
	} {
		if got := FaceOrderOf(m, typ); !slices.Equal(got, want) {
			t.Errorf("FaceOrderOf(%s) = %v, want %v", typ, got, want)
		}
	}
}

// TestDeclOrder_AnchorsInIncludedFile pins the same resolution for an
// included file, whose order is recorded by a separate pass.
func TestDeclOrder_AnchorsInIncludedFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	createFile(t, filepath.Join(dir, "metamodel.yaml"), `
version: "1.0"
includes:
  - part.yaml
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties:
      title: {type: string}
`)
	createFile(t, filepath.Join(dir, "part.yaml"), `
entities:
  page:
    label: Page
    id_prefix: PAGE
    properties:
      title: {type: string}
    faces: &pf
      review: {}
      draft: {}
  memo:
    label: Memo
    id_prefix: MEMO
    properties:
      title: {type: string}
    faces:
      <<: *pf
      archived: {}
`)
	m, _, err := Load(filepath.Join(dir, "metamodel.yaml"), testMetaFS)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := FaceOrderOf(m, "memo"), []string{"review", "draft", "archived"}; !slices.Equal(got, want) {
		t.Errorf("FaceOrderOf(memo) = %v, want %v", got, want)
	}
}

// TestMappingEntries_ExplicitKeyWinsOverMerged pins the decoder's precedence
// for a key both merged and set explicitly: the explicit value, at the
// merged key's first position.
func TestMappingEntries_ExplicitKeyWinsOverMerged(t *testing.T) {
	t.Parallel()
	var root yaml.Node
	src := "base: &b {x: 1, y: 2}\nm:\n  <<: *b\n  y: 3\n  z: 4\n"
	if err := yaml.Unmarshal([]byte(src), &root); err != nil {
		t.Fatal(err)
	}
	doc, ok := documentMapping(&root)
	if !ok {
		t.Fatal("no document mapping")
	}
	m, _ := mappingValue(doc, "m")
	if got, want := mappingKeys(m), []string{"x", "y", "z"}; !slices.Equal(got, want) {
		t.Errorf("mappingKeys = %v, want %v", got, want)
	}
	if y, _ := mappingValue(m, "y"); y == nil || y.Value != "3" {
		t.Errorf("y = %v, want the explicit 3", y)
	}
}
