package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Version tags (TKT-VO6VG9) name one entity version, beside the history
// rather than inside the entity. The store keeps them ([store.VersionTagger]);
// this file is the write boundary: it authorizes, attributes and audits.
//
// They live on [VersionTags], not on Manager, for the reason CopiesForSource
// gives: Manager sits on its plimsoll method load line.

// VersionTagStore is the part of [store.VersionTagger] the tag writes call.
// Defined here at the consumer; the store's tagger satisfies it.
type VersionTagStore interface {
	TagCurrent(ctx context.Context, req store.TagRequest) (store.VersionMeta, error)
	TagVersion(ctx context.Context, req store.TagRequest) (store.VersionMeta, error)
	UntagVersion(ctx context.Context, req store.UntagRequest) error
}

// TagGuard answers whether the ctx principal holds a permission for one
// entity. Same shape as [CopyGuard] and statemachine.Guard, and satisfied by
// the same kind of wiring: a subject-aware check over the ACL that allows
// everything when no policy is configured.
type TagGuard interface {
	HoldsPermission(ctx context.Context, entityID, permission string) bool
}

// ErrVersionTagsUnsupported is returned by every [VersionTags] write when the
// storage backend keeps no version history. It wraps
// [store.ErrHistoryUnsupported], so callers that already recognize the
// history refusal recognize this one too.
var ErrVersionTagsUnsupported = fmt.Errorf(
	"%w (version tags need the PostgreSQL or SQLite build)", store.ErrHistoryUnsupported)

// VersionTags sets, moves and deletes version tags through the write path of
// one [Manager].
//
// Authorization, in order:
//
//  1. The manager's ACL for `update` on the entity, without naming fields: a
//     tag changes no property, so a principal whose update grant is
//     row-level may tag even when its field grants are partial.
//  2. For a namespaced name `ns/...`, the per-entity permission `tag:ns`
//     through the [TagGuard]. So an operator grants `tag:sync` to a sync
//     connector's identity, and a user who can edit the entity cannot move
//     its sync base.
//
// A denial of either check is audited as a denied write.
//
// allow_acl_bypass does not cover tags: the writer is built over the base
// manager, never an elevated or cascade handle ([NewVersionTags] refuses
// one), so a bypassing script still needs `update` and `tag:<ns>` to tag.
type VersionTags struct {
	m      *Manager
	tagger VersionTagStore
	guard  TagGuard
}

// NewVersionTags builds the tag writer over m.
//
// Nil: m is rejected. tagger is accepted and means the backend keeps no
// history: every write then returns [ErrVersionTagsUnsupported]. guard is
// accepted and makes every namespaced tag fail closed, matching the nil
// [CopyGuard] rule; default-namespace tags are unaffected.
//
// An elevated or cascade handle is rejected too: tags are authorized for the
// acting principal, never bypassed (see [VersionTags]).
func NewVersionTags(m *Manager, tagger VersionTagStore, guard TagGuard) (*VersionTags, error) {
	if m == nil {
		return nil, errors.New("entitymanager: NewVersionTags: Manager is required")
	}
	if m.bypassACL || m.cascadeWrite {
		return nil, errors.New("entitymanager: NewVersionTags: needs the base manager, not an elevated handle")
	}
	return &VersionTags{m: m, tagger: tagger, guard: guard}, nil
}

// CallerView reads one face as the calling surface shows it to its
// principal: row-gated and field-redacted. A face the principal may not read
// is [store.ErrNotFound]. The Lua surface passes its script reader; an
// operator shell passes the raw store read.
type CallerView = func(ctx context.Context, ref entity.Ref) (*entity.Entity, error)

// TagCurrent tags ref's current state with name.
//
// When expect is set the write is a compare-and-set, and the token is per
// reader: expect must equal [store.VersionOf] of view's read of ref, the read
// the caller minted it from. A redacted reader's token therefore works, and
// it ignores changes to fields that reader cannot see. The raw row is read
// first, then view's; a mismatch is a *[store.VersionConflictError] that
// never reaches the store. On a match the store gets the raw row's own token,
// so any write after the raw read still conflicts under the store's lock.
// Neither conflict reports the stored token: that would let a caller test
// guesses of hidden fields against it.
//
// view is required when expect is set and ignored otherwise. A face view
// cannot read fails like a missing entity.
func (v *VersionTags) TagCurrent(
	ctx context.Context, ref entity.Ref, name store.VersionTagName, expect store.EntityVersion, view CallerView,
) (store.VersionMeta, error) {
	var check func(*entity.Entity) error
	if expect != "" {
		check = func(*entity.Entity) error { return checkCallerToken(ctx, ref, expect, view) }
	}
	return v.tag(ctx, ref, name, check, func(raw *entity.Entity, p principal.Principal) (store.VersionMeta, error) {
		req := store.TagRequest{Ref: ref, Name: name, PrincipalUser: p.User, PrincipalTool: p.Tool}
		if expect == "" {
			return v.tagger.TagCurrent(ctx, req)
		}
		req.Expect = store.VersionOf(raw)
		meta, err := v.tagger.TagCurrent(ctx, req)
		if errors.Is(err, store.ErrConflict) {
			return store.VersionMeta{}, &store.VersionConflictError{ID: ref.ID, Expected: expect}
		}
		return meta, err
	})
}

// checkCallerToken is TagCurrent's caller-side compare: expect against
// view's read of ref.
func checkCallerToken(ctx context.Context, ref entity.Ref, expect store.EntityVersion, view CallerView) error {
	if view == nil {
		return errors.New("entitymanager: a version tag with an expected token needs the caller's view")
	}
	seen, err := view(ctx, ref)
	if errors.Is(err, store.ErrNotFound) || (err == nil && seen == nil) {
		return newEntityNotFound(ref.String())
	}
	if err != nil {
		return fmt.Errorf("entitymanager: read %s: %w", ref, err)
	}
	if actual := store.VersionOf(seen); actual != expect {
		return &store.VersionConflictError{ID: ref.ID, Expected: expect, Actual: actual}
	}
	return nil
}

// TagVersion tags the 1-based version ordinal version of ref with name. See
// [store.VersionTagger.TagVersion] for which versions can be tagged.
func (v *VersionTags) TagVersion(
	ctx context.Context, ref entity.Ref, name store.VersionTagName, version int,
) (store.VersionMeta, error) {
	return v.tag(ctx, ref, name, nil, func(_ *entity.Entity, p principal.Principal) (store.VersionMeta, error) {
		return v.tagger.TagVersion(ctx, store.TagRequest{
			Ref: ref, Name: name, Version: version, PrincipalUser: p.User, PrincipalTool: p.Tool,
		})
	})
}

// UntagVersion deletes the tag name of ref. A tag that does not exist is
// [store.ErrNotFound].
func (v *VersionTags) UntagVersion(ctx context.Context, ref entity.Ref, name store.VersionTagName) error {
	e, p, err := v.prepare(ctx, ref, name, nil)
	if err != nil {
		return err
	}
	if err := v.tagger.UntagVersion(ctx, store.UntagRequest{
		Ref: ref, Name: name, PrincipalUser: p.User, PrincipalTool: p.Tool,
	}); err != nil {
		return err
	}
	v.record(ctx, audit.OpUntagVersion, e, fmt.Sprintf("tag=%q", name.String()))
	return nil
}

// tag is the shared body of TagCurrent and TagVersion: prepare, write, audit.
func (v *VersionTags) tag(
	ctx context.Context, ref entity.Ref, name store.VersionTagName, check func(*entity.Entity) error,
	write func(raw *entity.Entity, p principal.Principal) (store.VersionMeta, error),
) (store.VersionMeta, error) {
	e, p, err := v.prepare(ctx, ref, name, check)
	if err != nil {
		return store.VersionMeta{}, err
	}
	meta, err := write(e, p)
	if err != nil {
		return store.VersionMeta{}, err
	}
	v.record(ctx, audit.OpTagVersion, e, fmt.Sprintf("tag=%q version=%d", name.String(), meta.Version))
	return meta, nil
}

// prepare runs the checks every tag write shares, in this order: backend
// support, the name, the principal, the stored row, check (when set),
// authorization. It returns the stored row (for the audit subject) and the
// acting principal. check runs before authorization so that a face the
// caller cannot see fails like a missing one rather than as a denial.
//
// The row is read raw, like every write-prep read: its type is the ACL
// subject. Surfaces that serve a caller resolve the address through their
// gated reader first, so a hidden entity reads as missing there.
func (v *VersionTags) prepare(
	ctx context.Context, ref entity.Ref, name store.VersionTagName, check func(*entity.Entity) error,
) (*entity.Entity, principal.Principal, error) {
	if v.tagger == nil {
		return nil, principal.Principal{}, ErrVersionTagsUnsupported
	}
	if name.IsZero() {
		return nil, principal.Principal{}, fmt.Errorf("%w: missing tag name", store.ErrInvalidVersionTag)
	}
	p, ok := principal.Stamped(ctx)
	if !ok || p.IsZero() || (p.User == principal.Unknown && p.Tool == principal.Unknown) {
		// The tag row records who set it. Recording an unknown author would
		// be the translated-placeholder attribution RR-U964M0 forbids.
		return nil, principal.Principal{}, errors.New(
			"entitymanager: a version tag write needs an identified principal")
	}
	e, err := v.m.deps.Store.GetEntity(ctx, ref)
	if errors.Is(err, store.ErrNotFound) {
		return nil, principal.Principal{}, newEntityNotFound(ref.String())
	}
	if err != nil {
		return nil, principal.Principal{}, fmt.Errorf("entitymanager: read %s: %w", ref, err)
	}
	if check != nil {
		if err := check(e); err != nil {
			return nil, principal.Principal{}, err
		}
	}
	if err := v.authorizeTag(ctx, e, name); err != nil {
		return nil, principal.Principal{}, err
	}
	return e, p, nil
}

// authorizeTag applies the two checks the [VersionTags] doc lists.
func (v *VersionTags) authorizeTag(ctx context.Context, e *entity.Entity, name store.VersionTagName) error {
	req := acl.WriteRequest{Op: acl.OpUpdate, Subject: acl.NewEntitySubject(e.Type, e.ID, e.Face)}
	if err := v.m.authorizeAndAudit(ctx, req); err != nil {
		return err
	}
	ns := name.Namespace()
	if ns == "" {
		return nil
	}
	perm := acl.TagPermission(ns)
	if v.guard != nil && v.guard.HoldsPermission(ctx, e.ID, perm) {
		return nil
	}
	decision := acl.Decision{
		RuleKind: "version-tag",
		RuleID:   perm,
		Reason:   fmt.Sprintf("tag %q requires permission %q on %q", name.String(), perm, e.ID),
	}
	if !isAffordanceProbe(ctx) {
		v.m.recordDeniedWrite(ctx, decision, req)
	}
	return &acl.ForbiddenError{Decision: decision}
}

// record writes the audit row for a successful tag write.
func (v *VersionTags) record(ctx context.Context, op string, e *entity.Entity, summary string) {
	if !e.Face.IsImplicit() {
		summary += fmt.Sprintf(" face=%q", e.Face)
	}
	v.m.deps.Audit.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          op,
		Subject:     &audit.Subject{Kind: "entity", Type: e.Type, ID: e.ID},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     summary,
	})
}
