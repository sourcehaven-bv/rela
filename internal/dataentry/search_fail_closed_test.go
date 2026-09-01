package dataentry

import (
	"context"
	"errors"
	"iter"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
)

// entityOnlySearcher is a VisibleSearcher that does NOT implement
// search.FieldVisibleSearcher — the shape a future decorator (cache, metrics,
// tracing) produces when it wraps the searcher without forwarding
// SearchVisibleFields.
type entityOnlySearcher struct{ served bool }

func (s *entityOnlySearcher) SearchVisible(
	_ context.Context, _ search.Query, _ map[string]search.TypeScope,
) iter.Seq2[search.Hit, error] {
	return func(yield func(search.Hit, error) bool) {
		// Serving even one hit here means un-redacted results reached the
		// caller — the oracle this path must refuse to open.
		s.served = true
		yield(search.Hit{ID: "TKT-001"}, nil)
	}
}

// hidingResolver reports that it can hide fields, so hidesAnyField() is true
// and redaction is genuinely in play.
type hidingResolver struct{}

func (hidingResolver) FieldVerdicts(context.Context, *entityPkg.Entity) FieldVerdicts {
	return FieldVerdicts{}
}
func (hidingResolver) RelationVerdicts(context.Context, *entityPkg.Entity) RelationVerdicts {
	return RelationVerdicts{}
}

// TestSearchVisibleHits_FailsClosedWithoutFieldSearcher pins TKT-NCLA67
// (gh#1093): when the ACL policy hides fields but the wired searcher cannot
// redact them, search must REFUSE rather than fall back to un-redacted results.
//
// The failure this prevents is silent by construction: a decorator wraps the
// searcher, the type assertion misses, and search stops redacting while the
// policy still hides — no error, no log, no test failure. The only signal would
// be a user seeing a value they should not.
//
// search.Visible.SearchVisibleFields already fails closed one layer down for
// the same reason ("silently skipping redaction is exactly the oracle this
// closes", RR-8W40EW). This is that principle applied to the outer seam.
func TestSearchVisibleHits_FailsClosedWithoutFieldSearcher(t *testing.T) {
	t.Parallel()

	vs := &entityOnlySearcher{}
	aff := affordanceService{
		acl:      func() acl.ACL { return acl.NopACL{} },
		resolver: func() FieldVerdictResolver { return hidingResolver{} },
	}

	var gotErr error
	var hits int
	for _, err := range searchVisibleHits(
		context.Background(), vs, aff, search.Query{Text: "anything"}, nil,
	) {
		if err != nil {
			gotErr = err
			break
		}
		hits++
	}

	if gotErr == nil {
		t.Fatalf("want an error when the searcher cannot redact; got %d hit(s) and no error", hits)
	}
	// ErrScope specifically: the caller (queryservice) maps it to
	// errACLListQuery, so it surfaces as an authorization failure rather than
	// a generic search error.
	if !errors.Is(gotErr, search.ErrScope) {
		t.Errorf("want search.ErrScope so the caller reports an ACL failure, got %v", gotErr)
	}
	if vs.served {
		t.Error("un-redacted hits were served before the refusal — the fallback must not run at all")
	}
}

// TestSearchVisibleHits_NoRedactionNeededStillWorks pins the other half: when
// the policy hides nothing, an entity-level-only searcher is CORRECT and must
// keep working.
//
// Without this, "fail closed whenever the searcher isn't a FieldVisibleSearcher"
// would also pass — and would break every deployment running the Nop resolver,
// where redaction is a provable no-op.
func TestSearchVisibleHits_NoRedactionNeededStillWorks(t *testing.T) {
	t.Parallel()

	vs := &entityOnlySearcher{}
	aff := affordanceService{
		acl:      func() acl.ACL { return acl.NopACL{} },
		resolver: func() FieldVerdictResolver { return NopFieldVerdictResolver{} },
	}

	var hits int
	for _, err := range searchVisibleHits(
		context.Background(), vs, aff, search.Query{Text: "anything"}, nil,
	) {
		if err != nil {
			t.Fatalf("no redaction is in play, so this must not error: %v", err)
		}
		hits++
	}
	if hits != 1 {
		t.Errorf("want the entity-level search to serve its hit, got %d", hits)
	}
}
