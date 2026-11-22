package visibility

import (
	"errors"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// A grant limited to some faces is decided against the faces a served row
// can have: covered is true, disjoint is false, anything else is refused
// (TKT-9BKVQ9I). A Cond has no face test, so nothing approximates it.
func TestGrantCond_Faces(t *testing.T) {
	t.Parallel()
	draft, final := entity.Face("draft"), entity.Face("final")
	when := &store.GraphQuery{FaceIn: []entity.Face{draft}}
	for _, tc := range []struct {
		name    string
		scope   []entity.Face
		want    bool
		refused bool
	}{
		{name: "scope inside the grant", scope: []entity.Face{draft}, want: true},
		{name: "scope outside the grant", scope: []entity.Face{final}, want: false},
		{name: "scope overlapping the grant", scope: []entity.Face{draft, final}, refused: true},
		{name: "every face", scope: nil, refused: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, err := grantCond(when, "doc", tc.scope)
			if tc.refused {
				if !errors.Is(err, errFieldCondUnsupported) {
					t.Fatalf("err = %v, want errFieldCondUnsupported", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if v, known := c.Constant(); !known || v != tc.want {
				t.Fatalf("got %v (known %v), want %v", v, known, tc.want)
			}
		})
	}
	if _, err := grantCond(&store.GraphQuery{EntityType: "other"}, "doc", nil); !errors.Is(err, errFieldCondUnsupported) {
		t.Fatalf("a grant for another type: err = %v", err)
	}
}

// visibleCond: an ungated property is true, a gated one no grant lists is
// false, and an unconditional grant makes it true.
func TestVisibleCond(t *testing.T) {
	t.Parallel()
	open := store.PropPredicate{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}
	fs := &search.FieldScope{
		Gated: []string{"secret", "title", "status"},
		Grants: []search.FieldGrant{
			{Fields: []string{"title"}},
			{Fields: []string{"secret"}, When: &store.GraphQuery{Props: []store.PropPredicate{open}}},
		},
	}
	for _, tc := range []struct {
		prop        string
		known, want bool
	}{
		{prop: "body", known: true, want: true},
		{prop: "title", known: true, want: true},
		{prop: "status", known: true, want: false},
		{prop: "secret", known: false},
	} {
		c, err := visibleCond(fs, "ticket", tc.prop, nil)
		if err != nil {
			t.Fatal(err)
		}
		if v, known := c.Constant(); known != tc.known || (known && v != tc.want) {
			t.Fatalf("%s: got %v (known %v)", tc.prop, v, known)
		}
	}
}

// guardHiddenProps guards each same-property subtree once, folds a known
// visibility, and refuses a comparison whose unset answer it cannot state.
func TestGuardHiddenProps(t *testing.T) {
	t.Parallel()
	v := store.PropLeaf(store.PropPredicate{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true})
	ge := store.PropPredicate{Property: "prio", Op: store.PropGreaterEqual, Value: "3", Kind: store.OrderInteger}
	le := ge
	le.Op = store.PropLessEqual
	conds := map[string]store.Cond{"prio": v, "secret": store.OrCond()}

	props, where, err := guardHiddenProps([]store.PropPredicate{ge, le}, nil, conds)
	if err != nil {
		t.Fatal(err)
	}
	if len(props) != 0 || where == nil {
		t.Fatalf("props %v, where %v", props, where)
	}
	guards := 0
	where.Walk(func(n store.Cond) bool {
		if n.Op == store.CondProp && n.Prop.Property == "status" {
			guards++
		}
		return true
	})
	if guards != 1 {
		t.Fatalf("the visibility condition appears %d times, want once", guards)
	}

	ne := store.PropPredicate{Property: "secret", Op: store.PropNotEqualOrEmpty, Value: "s", Scalar: true}
	_, where, err = guardHiddenProps([]store.PropPredicate{ne}, nil, conds)
	if err != nil {
		t.Fatal(err)
	}
	if c, known := where.Constant(); !known || !c {
		t.Fatalf("`~=` on a field hidden everywhere is true on every row, got %v", where)
	}

	keyed := store.PropPredicate{Property: "secret", Op: store.PropKeyEqual, Key: "k", Value: "v"}
	if _, _, err := guardHiddenProps([]store.PropPredicate{keyed}, nil, conds); !errors.Is(err, errFieldCondUnsupported) {
		t.Fatalf("an unknown comparison: err = %v", err)
	}
}

func TestReturnableFaces(t *testing.T) {
	t.Parallel()
	draft, final := entity.Face("draft"), entity.Face("final")
	world := func(fb store.Fallback) store.FaceSelection {
		return store.InWorld(store.NewWorldScope(map[string]store.TypeResolution{
			"doc": {Chain: []entity.Face{draft}, Fallback: fb},
		}))
	}
	for _, tc := range []struct {
		name  string
		scope []entity.Face
		sel   store.FaceSelection
		want  []entity.Face
	}{
		{"every face, no scope", nil, store.AllFaces(), nil},
		{"every face, scoped", []entity.Face{draft}, store.AllFaces(), []entity.Face{draft}},
		{"face set, no scope", nil, store.AtFaces(draft, final), []entity.Face{draft, final}},
		{"face set meets scope", []entity.Face{final}, store.AtFaces(draft, final), []entity.Face{final}},
		{"disjoint", []entity.Face{final}, store.AtFaces(draft), []entity.Face{}},
		{"world chain", nil, world(store.FallbackExclude), []entity.Face{draft}},
		{"world with a default fallback is unknown", nil, world(store.FallbackDefaultState), nil},
		{"unscoped type in a world", nil, store.InWorld(store.TrivialScope()), []entity.Face{entity.ImplicitFace}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := returnableFaces(tc.scope, tc.sel, "doc")
			if !slices.Equal(got, tc.want) || (got == nil) != (tc.want == nil) {
				t.Errorf("got %#v, want %#v", got, tc.want)
			}
		})
	}
}
