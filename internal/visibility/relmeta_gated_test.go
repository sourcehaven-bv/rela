package visibility_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// gatedRedactor declares meta grants on one relation type.
type gatedRedactor struct {
	visibility.NopRedactor
	gated string
}

func (g gatedRedactor) RelationMetaGated(relType string) bool { return relType == g.gated }

func TestRelationMetaGatedBy(t *testing.T) {
	st := memstore.New()
	script := func(r visibility.FieldRedactor) *visibility.ScriptReader {
		t.Helper()
		policy, err := visibility.NewPolicyReader(idGate{id: "TKT-1"}, r, st)
		if err != nil {
			t.Fatal(err)
		}
		sr, err := visibility.NewScriptReader(policy, st, nil)
		if err != nil {
			t.Fatal(err)
		}
		return sr
	}
	allow, err := visibility.NewAllowAllReader(st)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		reader any
		want   bool
	}{
		{"nil", nil, true},
		{"nil unrestricted", (*visibility.UnrestrictedReader)(nil), true},
		{"unrestricted", visibility.Unrestricted(st), false},
		{"nil script", (*visibility.ScriptReader)(nil), true},
		{"script, no grants", script(visibility.NopRedactor{}), false},
		{"script, grants on the type", script(gatedRedactor{gated: "blocks"}), true},
		{"script, grants on another type", script(gatedRedactor{gated: "cites"}), false},
		{"another reader", allow, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := visibility.RelationMetaGatedBy(tc.reader, "blocks"); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
