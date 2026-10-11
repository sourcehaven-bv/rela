package entitymanager_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// fakeTagger records the requests the tag writer sends to the store.
type fakeTagger struct {
	tags    []store.TagRequest
	untags  []store.UntagRequest
	err     error
	version int
}

func (f *fakeTagger) TagCurrent(_ context.Context, req store.TagRequest) (store.VersionMeta, error) {
	f.tags = append(f.tags, req)
	return store.VersionMeta{Version: f.version}, f.err
}

func (f *fakeTagger) TagVersion(_ context.Context, req store.TagRequest) (store.VersionMeta, error) {
	f.tags = append(f.tags, req)
	return store.VersionMeta{Version: req.Version}, f.err
}

func (f *fakeTagger) UntagVersion(_ context.Context, req store.UntagRequest) error {
	f.untags = append(f.untags, req)
	return f.err
}

// fakeTagGuard answers the namespace check from a fixed set and records the
// permissions it was asked about.
type fakeTagGuard struct {
	granted map[string]bool
	asked   []string
}

func (g *fakeTagGuard) HoldsPermission(_ context.Context, _, permission string) bool {
	g.asked = append(g.asked, permission)
	return g.granted[permission]
}

// denyAllFieldGate refuses every field write, to show a tag writes none.
type denyAllFieldGate struct{}

func (denyAllFieldGate) CheckFieldWrite(context.Context, *entity.Entity, map[string]any, []string) error {
	return errors.New("field write denied")
}

var alice = principal.Principal{User: "alice", Tool: principal.ToolCLI}

func taggerCtx() context.Context {
	return principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolCLI})
}

func mustTagName(t *testing.T, s string) store.VersionTagName {
	t.Helper()
	n, err := store.ParseVersionTagName(s)
	if err != nil {
		t.Fatalf("ParseVersionTagName(%q): %v", s, err)
	}
	return n
}

func countOps(recs []audit.Record, op string) int {
	n := 0
	for _, r := range recs {
		if r.Op == op {
			n++
		}
	}
	return n
}

func TestNewVersionTags_RejectsNilManager(t *testing.T) {
	t.Parallel()
	if _, err := entitymanager.NewVersionTags(nil, &fakeTagger{}, &fakeTagGuard{}); err == nil {
		t.Fatal("NewVersionTags(nil manager): want error")
	}
}

// TestNewVersionTags_RejectsElevated pins that allow_acl_bypass does not
// cover tags: the writer cannot be built over a bypassing handle.
func TestNewVersionTags_RejectsElevated(t *testing.T) {
	t.Parallel()
	mgr, _ := newManagerWithACL(t, acl.NopACL{}, audit.Nop{})
	elevated, ok := mgr.Elevated().(*entitymanager.Manager)
	if !ok {
		t.Fatal("Elevated() is not a *Manager")
	}
	if _, err := entitymanager.NewVersionTags(elevated, &fakeTagger{}, &fakeTagGuard{}); err == nil {
		t.Fatal("NewVersionTags(elevated manager): want error")
	}
}

// TestVersionTags_Authorization covers every path through prepare and
// authorizeTag: the refusals before the ACL, the update check and the
// namespace check.
func TestVersionTags_Authorization(t *testing.T) {
	t.Parallel()
	ref := entity.Ref{ID: "DEC-001"}
	tests := []struct {
		name       string
		acl        acl.ACL
		nilTagger  bool
		guard      *fakeTagGuard
		nilGuard   bool
		unstamped  bool
		who        principal.Principal
		tag        string
		ref        entity.Ref
		wantErr    func(error) bool
		wantDenied int
		wantTagged bool
		wantAsked  []string
	}{
		{
			name: "default namespace allowed by update", acl: acl.NopACL{}, who: alice,
			tag: "reviewed", ref: ref, wantTagged: true,
		},
		{
			name: "no tagger is unsupported", acl: acl.NopACL{}, nilTagger: true, who: alice,
			tag: "reviewed", ref: ref,
			wantErr: func(err error) bool { return errors.Is(err, store.ErrHistoryUnsupported) },
		},
		{
			name: "unstamped principal refused", acl: acl.NopACL{}, unstamped: true,
			tag: "reviewed", ref: ref,
			wantErr: func(err error) bool { return err != nil && strings.Contains(err.Error(), "principal") },
		},
		{
			name: "unknown principal refused", acl: acl.NopACL{},
			who: principal.Principal{User: principal.Unknown, Tool: principal.Unknown},
			tag: "reviewed", ref: ref,
			wantErr: func(err error) bool { return err != nil && strings.Contains(err.Error(), "principal") },
		},
		{
			name: "missing entity", acl: acl.NopACL{}, who: alice,
			tag: "reviewed", ref: entity.Ref{ID: "DEC-999"},
			wantErr: func(err error) bool { return errors.Is(err, entitymanager.ErrEntityNotFound) },
		},
		{
			name: "update denied", acl: acl.ReadOnlyACL{}, who: alice,
			tag: "reviewed", ref: ref, wantDenied: 1,
			wantErr: func(err error) bool { var f *acl.ForbiddenError; return errors.As(err, &f) },
		},
		{
			name: "namespace granted", acl: acl.NopACL{}, who: alice,
			guard: &fakeTagGuard{granted: map[string]bool{"tag:sync": true}},
			tag:   "sync/jira", ref: ref, wantTagged: true, wantAsked: []string{"tag:sync"},
		},
		{
			name: "namespace denied", acl: acl.NopACL{}, who: alice,
			guard: &fakeTagGuard{granted: map[string]bool{"tag:other": true}},
			tag:   "sync/jira", ref: ref, wantDenied: 1, wantAsked: []string{"tag:sync"},
			wantErr: func(err error) bool {
				var f *acl.ForbiddenError
				return errors.As(err, &f) && f.Decision.RuleID == "tag:sync"
			},
		},
		{
			name: "nil guard fails closed for a namespace", acl: acl.NopACL{}, nilGuard: true, who: alice,
			tag: "sync/jira", ref: ref, wantDenied: 1,
			wantErr: func(err error) bool { var f *acl.ForbiddenError; return errors.As(err, &f) },
		},
		{
			name: "update denial stops before the namespace check", acl: acl.ReadOnlyACL{}, who: alice,
			guard: &fakeTagGuard{granted: map[string]bool{"tag:sync": true}},
			tag:   "sync/jira", ref: ref, wantDenied: 1,
			wantErr: func(err error) bool { var f *acl.ForbiddenError; return errors.As(err, &f) },
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sink := audit.NewMemory()
			mgr, cs := newManagerWithACL(t, tc.acl, sink)
			seedEntity(t, cs, "decision", "A decision")
			tagger := &fakeTagger{version: 3}
			var st entitymanager.VersionTagStore = tagger
			if tc.nilTagger {
				st = nil
			}
			guard := tc.guard
			if guard == nil {
				guard = &fakeTagGuard{}
			}
			var g entitymanager.TagGuard = guard
			if tc.nilGuard {
				g = nil
			}
			vt, err := entitymanager.NewVersionTags(mgr, st, g)
			if err != nil {
				t.Fatalf("NewVersionTags: %v", err)
			}

			ctx := context.Background()
			if !tc.unstamped {
				ctx = principal.With(ctx, tc.who)
			}
			meta, err := vt.TagCurrent(ctx, tc.ref, mustTagName(t, tc.tag), "", nil)
			if tc.wantErr != nil {
				if !tc.wantErr(err) {
					t.Fatalf("TagCurrent error = %v, not the expected kind", err)
				}
			} else if err != nil {
				t.Fatalf("TagCurrent: %v", err)
			}
			if got := len(tagger.tags) > 0; got != tc.wantTagged {
				t.Errorf("store called = %v, want %v", got, tc.wantTagged)
			}
			if tc.wantTagged && meta.Version != 3 {
				t.Errorf("meta.Version = %d, want 3", meta.Version)
			}
			recs := sink.Records()
			if got := countOps(recs, audit.OpDeniedWrite); got != tc.wantDenied {
				t.Errorf("denied-write rows = %d, want %d", got, tc.wantDenied)
			}
			wantTagRows := 0
			if tc.wantTagged {
				wantTagRows = 1
			}
			if got := countOps(recs, audit.OpTagVersion); got != wantTagRows {
				t.Errorf("version-tag rows = %d, want %d", got, wantTagRows)
			}
			if strings.Join(guard.asked, ",") != strings.Join(tc.wantAsked, ",") {
				t.Errorf("guard asked %v, want %v", guard.asked, tc.wantAsked)
			}
		})
	}
}

// TestVersionTags_NoFieldWrite pins R12: a tag changes no property, so a
// principal whose field grants refuse every field may still tag.
func TestVersionTags_NoFieldWrite(t *testing.T) {
	t.Parallel()
	cs := &countingStore{Store: memstore.New()}
	seedEntity(t, cs, "decision", "A decision")
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: cs, Meta: parseMeta(t), Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: denyAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	tagger := &fakeTagger{}
	vt, err := entitymanager.NewVersionTags(entitymanager.FieldGated(mgr), tagger, &fakeTagGuard{})
	if err != nil {
		t.Fatalf("NewVersionTags: %v", err)
	}
	if _, err := vt.TagCurrent(taggerCtx(), entity.Ref{ID: "DEC-001"}, mustTagName(t, "reviewed"), "", nil); err != nil {
		t.Fatalf("TagCurrent under a deny-all field gate: %v", err)
	}
}

// TestVersionTags_RequestsAndAudit checks what reaches the store and the
// audit log for each of the three writes.
func TestVersionTags_RequestsAndAudit(t *testing.T) {
	t.Parallel()
	sink := audit.NewMemory()
	mgr, cs := newManagerWithACL(t, acl.NopACL{}, sink)
	seedEntity(t, cs, "decision", "A decision")
	tagger := &fakeTagger{version: 2}
	vt, err := entitymanager.NewVersionTags(mgr, tagger, &fakeTagGuard{})
	if err != nil {
		t.Fatalf("NewVersionTags: %v", err)
	}
	ctx := audit.WithTriggeredBy(taggerCtx(), "schedule:nightly")
	ref := entity.Ref{ID: "DEC-001"}
	name := mustTagName(t, "reviewed")

	raw, err := cs.GetEntity(ctx, ref)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	tok := store.VersionOf(raw)
	if _, err := vt.TagCurrent(ctx, ref, name, tok, cs.GetEntity); err != nil {
		t.Fatalf("TagCurrent: %v", err)
	}
	if _, err := vt.TagVersion(ctx, ref, name, 1); err != nil {
		t.Fatalf("TagVersion: %v", err)
	}
	if err := vt.UntagVersion(ctx, ref, name); err != nil {
		t.Fatalf("UntagVersion: %v", err)
	}

	if len(tagger.tags) != 2 || len(tagger.untags) != 1 {
		t.Fatalf("store calls: %d tags, %d untags; want 2, 1", len(tagger.tags), len(tagger.untags))
	}
	cur, ver := tagger.tags[0], tagger.tags[1]
	if cur.Expect != tok || cur.Version != 0 || cur.PrincipalUser != "alice" || cur.PrincipalTool != principal.ToolCLI {
		t.Errorf("TagCurrent request = %+v", cur)
	}
	if ver.Version != 1 || ver.Expect != "" || ver.Ref != ref || ver.Name != name {
		t.Errorf("TagVersion request = %+v", ver)
	}
	if un := tagger.untags[0]; un.PrincipalUser != "alice" || un.Ref != ref {
		t.Errorf("UntagVersion request = %+v", un)
	}

	var tagRows, untagRows []audit.Record
	for _, r := range sink.Records() {
		switch r.Op {
		case audit.OpTagVersion:
			tagRows = append(tagRows, r)
		case audit.OpUntagVersion:
			untagRows = append(untagRows, r)
		}
	}
	if len(tagRows) != 2 || len(untagRows) != 1 {
		t.Fatalf("audit rows: %d tag, %d untag; want 2, 1", len(tagRows), len(untagRows))
	}
	for _, r := range append(tagRows, untagRows...) {
		if r.Subject == nil || r.Subject.ID != "DEC-001" || r.Subject.Type != "decision" {
			t.Errorf("%s subject = %+v", r.Op, r.Subject)
		}
		if r.Principal.User != "alice" || r.TriggeredBy != "schedule:nightly" {
			t.Errorf("%s attribution = %+v / %q", r.Op, r.Principal, r.TriggeredBy)
		}
	}
	if got := tagRows[1].Summary; got != `tag="reviewed" version=1` {
		t.Errorf("tag summary = %q", got)
	}
	if got := untagRows[0].Summary; got != `tag="reviewed"` {
		t.Errorf("untag summary = %q", got)
	}
}

// TestVersionTags_StoreErrorsPassThrough checks that a refused store write is
// returned unchanged and leaves no audit row.
func TestVersionTags_StoreErrorsPassThrough(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
	}{
		{"conflict", &store.VersionConflictError{}},
		{"missing tag", store.ErrNotFound},
		{"in transaction", store.ErrTagInTx},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			sink := audit.NewMemory()
			mgr, cs := newManagerWithACL(t, acl.NopACL{}, sink)
			seedEntity(t, cs, "decision", "A decision")
			vt, err := entitymanager.NewVersionTags(mgr, &fakeTagger{err: tc.err}, &fakeTagGuard{})
			if err != nil {
				t.Fatalf("NewVersionTags: %v", err)
			}
			_, err = vt.TagCurrent(taggerCtx(), entity.Ref{ID: "DEC-001"}, mustTagName(t, "reviewed"), "", nil)
			if !errors.Is(err, tc.err) {
				t.Errorf("TagCurrent error = %v, want %v", err, tc.err)
			}
			if uerr := vt.UntagVersion(taggerCtx(), entity.Ref{ID: "DEC-001"}, mustTagName(t, "reviewed")); !errors.Is(uerr, tc.err) {
				t.Errorf("UntagVersion error = %v, want %v", uerr, tc.err)
			}
			recs := sink.Records()
			if countOps(recs, audit.OpTagVersion)+countOps(recs, audit.OpUntagVersion) != 0 {
				t.Error("a failed tag write was audited")
			}
		})
	}
}

// txProbeStore records whether the ctx a manager uses inside a store
// transaction is marked with store.ContextInTx. A version tag write refuses
// on a marked ctx (the store side is pinned by storetest), so the marking is
// what keeps a tag out of an open transaction.
type txProbeStore struct {
	store.Store
	marked []bool
}

func (p *txProbeStore) Tx(ctx context.Context, fn func(store.Store) error) error {
	return p.Store.Tx(ctx, func(view store.Store) error {
		return fn(&txProbeView{Store: view, probe: p})
	})
}

type txProbeView struct {
	store.Store
	probe *txProbeStore
}

func (v *txProbeView) DeleteFamily(ctx context.Context, id string, cascade bool) (*store.DeleteResult, error) {
	v.probe.marked = append(v.probe.marked, store.InTx(ctx))
	return v.Store.DeleteFamily(ctx, id, cascade)
}

func TestManager_TxCallbacksMarkContext(t *testing.T) {
	t.Parallel()
	probe := &txProbeStore{Store: memstore.New()}
	cs := &countingStore{Store: probe}
	seedEntity(t, cs, "decision", "A decision")
	mgr, err := entitymanager.New(entitymanager.Deps{
		Store: cs, Meta: parseMeta(t), Templater: nopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
	})
	if err != nil {
		t.Fatalf("entitymanager.New: %v", err)
	}
	if store.InTx(taggerCtx()) {
		t.Fatal("an unmarked ctx reads as in a transaction")
	}
	if _, err := mgr.DeleteEntity(taggerCtx(), "DEC-001", false); err != nil {
		t.Fatalf("DeleteEntity: %v", err)
	}
	if len(probe.marked) == 0 {
		t.Fatal("DeleteEntity did not delete inside a transaction")
	}
	for i, m := range probe.marked {
		if !m {
			t.Errorf("delete %d ran in a transaction on an unmarked ctx", i)
		}
	}
}

// casTagger is a VersionTagStore whose TagCurrent applies the store's
// compare-and-set to the live row, like the real backends do under their
// lock.
type casTagger struct {
	fakeTagger
	st store.Store
}

func (c *casTagger) TagCurrent(ctx context.Context, req store.TagRequest) (store.VersionMeta, error) {
	c.tags = append(c.tags, req)
	if req.Expect != "" {
		live, err := c.st.GetEntity(ctx, req.Ref)
		if err != nil {
			return store.VersionMeta{}, err
		}
		if actual := store.VersionOf(live); actual != req.Expect {
			return store.VersionMeta{}, &store.VersionConflictError{ID: req.Ref.ID, Expected: req.Expect, Actual: actual}
		}
	}
	return store.VersionMeta{Version: 1}, nil
}

// redactingView is a caller view that withholds the property "secret", as a
// principal with partial field visibility reads the entity.
func redactingView(st store.Store) entitymanager.CallerView {
	return func(ctx context.Context, ref entity.Ref) (*entity.Entity, error) {
		e, err := st.GetEntity(ctx, ref)
		if err != nil {
			return nil, err
		}
		e = e.Clone()
		delete(e.Properties, "secret")
		return e, nil
	}
}

// seedSecretDecision seeds DEC-001 with a property the redacting view hides.
func seedSecretDecision(t *testing.T) (*entitymanager.VersionTags, *casTagger, *countingStore) {
	t.Helper()
	mgr, cs := newManagerWithACL(t, acl.NopACL{}, audit.Nop{})
	seedEntity(t, cs, "decision", "A decision")
	setStoredProp(context.Background(), t, cs, "secret", "s3cret")
	tagger := &casTagger{st: cs}
	vt, err := entitymanager.NewVersionTags(mgr, tagger, &fakeTagGuard{})
	if err != nil {
		t.Fatalf("NewVersionTags: %v", err)
	}
	return vt, tagger, cs
}

func setStoredProp(ctx context.Context, t *testing.T, st store.Store, key, value string) {
	t.Helper()
	e, err := st.GetEntity(ctx, entity.Ref{ID: "DEC-001"})
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	e = e.Clone()
	e.SetString(key, value)
	if err := st.UpdateEntity(ctx, e); err != nil {
		t.Fatalf("UpdateEntity: %v", err)
	}
}

// TestVersionTags_TokenIsPerReader: a reader that cannot see every field tags
// with the token of its own read, and the store is handed the raw row's
// token.
func TestVersionTags_TokenIsPerReader(t *testing.T) {
	t.Parallel()
	vt, tagger, cs := seedSecretDecision(t)
	ref := entity.Ref{ID: "DEC-001"}
	view := redactingView(cs)
	seen, err := view(context.Background(), ref)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	if _, tagErr := vt.TagCurrent(taggerCtx(), ref, mustTagName(t, "reviewed"), store.VersionOf(seen), view); tagErr != nil {
		t.Fatalf("TagCurrent with the redacted reader's own token: %v", tagErr)
	}
	raw, err := cs.GetEntity(context.Background(), ref)
	if err != nil {
		t.Fatalf("GetEntity: %v", err)
	}
	if len(tagger.tags) != 1 || tagger.tags[0].Expect != store.VersionOf(raw) {
		t.Fatalf("store requests = %+v, want one with the raw row's token", tagger.tags)
	}
}

// TestVersionTags_TokenIsNoHiddenFieldOracle: a token computed over a guessed
// hidden value, even the right guess, conflicts exactly like a wrong token,
// so the compare-and-set cannot confirm a value the caller cannot see.
func TestVersionTags_TokenIsNoHiddenFieldOracle(t *testing.T) {
	t.Parallel()
	vt, tagger, cs := seedSecretDecision(t)
	ref := entity.Ref{ID: "DEC-001"}
	view := redactingView(cs)
	seen, err := view(context.Background(), ref)
	if err != nil {
		t.Fatalf("view: %v", err)
	}
	guess := func(v string) store.EntityVersion {
		g := seen.Clone()
		g.SetString("secret", v)
		return store.VersionOf(g)
	}
	var results []string
	for _, tok := range []store.EntityVersion{guess("s3cret"), guess("wrong"), "garbage"} {
		_, err := vt.TagCurrent(taggerCtx(), ref, mustTagName(t, "reviewed"), tok, view)
		var conflict *store.VersionConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("TagCurrent(%q) = %v, want a version conflict", tok, err)
		}
		if conflict.Expected != tok {
			t.Errorf("conflict.Expected = %q, want %q", conflict.Expected, tok)
		}
		results = append(results, string(conflict.Actual))
	}
	if results[0] != results[1] || results[1] != results[2] {
		t.Errorf("conflicts differ by guess: %q", results)
	}
	if len(tagger.tags) != 0 {
		t.Errorf("a mismatched token reached the store: %+v", tagger.tags)
	}
}

// TestVersionTags_WriteAfterTokenConflicts: a write between the token and the
// tag conflicts, whether the caller can see the changed field (its own
// compare fails) or not, provided the write lands after the raw read (the
// store's compare fails, and reports no stored token).
func TestVersionTags_WriteAfterTokenConflicts(t *testing.T) {
	t.Parallel()
	ref := entity.Ref{ID: "DEC-001"}

	t.Run("visible field changed after the token", func(t *testing.T) {
		t.Parallel()
		vt, tagger, cs := seedSecretDecision(t)
		view := redactingView(cs)
		seen, err := view(context.Background(), ref)
		if err != nil {
			t.Fatalf("view: %v", err)
		}
		tok := store.VersionOf(seen)
		setStoredProp(context.Background(), t, cs, "title", "Edited")
		_, err = vt.TagCurrent(taggerCtx(), ref, mustTagName(t, "reviewed"), tok, view)
		if !errors.Is(err, store.ErrConflict) {
			t.Fatalf("TagCurrent = %v, want a conflict", err)
		}
		if len(tagger.tags) != 0 {
			t.Errorf("store reached: %+v", tagger.tags)
		}
	})

	t.Run("hidden field changed after the raw read", func(t *testing.T) {
		t.Parallel()
		vt, tagger, cs := seedSecretDecision(t)
		inner := redactingView(cs)
		seen, err := inner(context.Background(), ref)
		if err != nil {
			t.Fatalf("view: %v", err)
		}
		tok := store.VersionOf(seen)
		// The manager reads raw before it calls the view, so a write made
		// inside the view lands between the raw read and the store's lock.
		racing := func(ctx context.Context, r entity.Ref) (*entity.Entity, error) {
			setStoredProp(ctx, t, cs, "secret", "rotated")
			return inner(ctx, r)
		}
		_, err = vt.TagCurrent(taggerCtx(), ref, mustTagName(t, "reviewed"), tok, racing)
		var conflict *store.VersionConflictError
		if !errors.As(err, &conflict) {
			t.Fatalf("TagCurrent = %v, want a version conflict", err)
		}
		if conflict.Actual != "" || conflict.Expected != tok {
			t.Errorf("conflict = %+v, want the caller's token and no stored token", conflict)
		}
		if len(tagger.tags) != 1 {
			t.Errorf("store requests = %d, want 1", len(tagger.tags))
		}
	})
}

// TestVersionTags_ExpectOnHiddenEntity: a face the caller's view cannot read
// fails like a missing entity, before authorization.
func TestVersionTags_ExpectOnHiddenEntity(t *testing.T) {
	t.Parallel()
	sink := audit.NewMemory()
	mgr, cs := newManagerWithACL(t, acl.ReadOnlyACL{}, sink)
	seedEntity(t, cs, "decision", "A decision")
	vt, err := entitymanager.NewVersionTags(mgr, &fakeTagger{}, &fakeTagGuard{})
	if err != nil {
		t.Fatalf("NewVersionTags: %v", err)
	}
	hidden := func(context.Context, entity.Ref) (*entity.Entity, error) { return nil, store.ErrNotFound }
	_, err = vt.TagCurrent(taggerCtx(), entity.Ref{ID: "DEC-001"}, mustTagName(t, "reviewed"), "tok", hidden)
	if !errors.Is(err, entitymanager.ErrEntityNotFound) {
		t.Fatalf("TagCurrent on a hidden entity = %v, want entity not found", err)
	}
	if n := countOps(sink.Records(), audit.OpDeniedWrite); n != 0 {
		t.Errorf("denied-write rows = %d, want 0 (hidden reads as missing)", n)
	}
	if _, err := vt.TagCurrent(taggerCtx(), entity.Ref{ID: "DEC-001"}, mustTagName(t, "reviewed"), "tok", nil); err == nil {
		t.Error("TagCurrent with expect and no view: want error")
	}
}
