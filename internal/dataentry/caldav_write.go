package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/caldavalias"
	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// PutCalendarObject applies a client write: an update to a mapped entity, or a
// create when the resource is new.
//
// The write goes through entitymanager.PatchEntity, never a read-modify-write.
// A CalDAV PUT carries a PARTIAL view — Apple sends SUMMARY, STATUS, COMPLETED,
// PERCENT-COMPLETE and DUE, and drops everything it does not model — so saving
// it as a whole entity would erase every property VTODO has no slot for,
// including ones the caller cannot even read.
func (b *caldavBackend) PutCalendarObject(
	ctx context.Context, p string, cal *ical.Calendar, opts *caldav.PutCalendarObjectOptions,
) (*caldav.CalendarObject, error) {
	name, href, ok := b.splitPath(p)
	if !ok || href == "" {
		return nil, webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: not found"))
	}
	m, cfg, err := b.mapperFor(name)
	if err != nil {
		return nil, err
	}

	in, err := todoFromICal(cal)
	if err != nil {
		return nil, webdav.NewHTTPError(http.StatusBadRequest, err)
	}

	existing, hasExisting := b.app.caldavAliases.Lookup(aliasPrincipal(ctx), name, href)
	if err := b.checkPreconditions(ctx, opts, name, href, m, existing, hasExisting); err != nil {
		return nil, err
	}

	entityID := ""
	switch {
	case hasExisting:
		entityID = existing.EntityID
	default:
		// A resource we have no alias for. It may still correspond to an
		// existing entity if the client is writing back a to-do we served, in
		// which case the UID carries our own <type>--<id>@rela form.
		if t, id, split := splitFeedUID(in.Todo.UID); split && t == cfg.EntityType {
			entityID = id
		}
	}

	if entityID == "" {
		// No alias and no rela-shaped UID: nothing on this server has ever
		// corresponded to this resource, so it is a genuine client-composed
		// create.
		return b.createFromTodo(ctx, name, href, m, in)
	}
	return b.updateFromTodo(ctx, name, href, m, in, entityID)
}

// checkPreconditions enforces If-Match / If-None-Match.
//
// Returning 412 on a stale If-Match is what stops a client that has been
// offline from overwriting a change made elsewhere.
//
// The comparison is against the ETag the resource WOULD BE SERVED RIGHT NOW,
// re-rendered here, not against any cached copy. Caching it in the alias table
// looked like a cheap way to skip the render, but that value is refreshed only
// by CalDAV writes — so any rela-side edit (the SPA, the CLI, MCP, an
// automation, a git pull) leaves it stale, and the two ETags drift.
//
// Both consequences are data-loss bugs, and both were reproduced:
//   - the client presents the tag the server JUST served it and gets a
//     permanent 412, because the stored copy is older. Retrying cannot help:
//     only a successful CalDAV write refreshes the stored tag, and that is
//     precisely what is being refused. The client wedges.
//   - a client holding the STALE tag succeeds, silently overwriting the newer
//     rela-side edit — the exact overwrite this precondition exists to stop.
//
// This is the same rule renderObject states for the render itself: two call
// sites deriving a conditional-request value independently is how conditional
// requests start failing for no visible reason.
func (b *caldavBackend) checkPreconditions(
	ctx context.Context, opts *caldav.PutCalendarObjectOptions,
	collection, href string, m *caldavMapper, existing caldavalias.Alias, hasExisting bool,
) error {
	if opts == nil {
		return nil
	}
	if opts.IfNoneMatch.IsSet() && hasExisting {
		// "*" or any etag: the client asserted the resource is absent.
		return webdav.NewHTTPError(http.StatusPreconditionFailed,
			errors.New("caldav: resource already exists"))
	}
	if !opts.IfMatch.IsSet() {
		return nil
	}
	want, etagErr := opts.IfMatch.ETag()
	if etagErr != nil {
		return webdav.NewHTTPError(http.StatusBadRequest, etagErr)
	}
	// No alias does NOT mean no resource. A to-do rela has served but that no
	// client has yet written back has no alias — it is addressed by the DERIVED
	// href (<type>--<id>@rela.ics), and objectFor mints that on the fly. Failing
	// the precondition here made every such resource permanently unwritable: the
	// client holds a valid ETag, gets 412, and cannot recover, because only a
	// successful write would create the alias that lets the check pass.
	//
	// Reproduced against Apple Reminders: toggling a never-before-synced to-do
	// reverted every time, with the client retrying the same doomed PUT.
	if !hasExisting {
		derivedUID := strings.TrimSuffix(href, ".ics")
		t, id, ok := splitFeedUID(derivedUID)
		if !ok || t != m.cfg.EntityType {
			// Not a derived href either, so nothing exists at this path: the
			// client asserted a precondition against a resource that is not there.
			return webdav.NewHTTPError(http.StatusPreconditionFailed,
				errors.New("caldav: etag mismatch"))
		}
		existing = caldavalias.Alias{
			Collection: collection, Href: href, UID: derivedUID, EntityID: id,
		}
	}
	current, err := b.currentETag(ctx, collection, m, existing)
	if err != nil {
		return err
	}
	if strings.Trim(current, `"`) != strings.Trim(want, `"`) {
		return webdav.NewHTTPError(http.StatusPreconditionFailed,
			errors.New("caldav: etag mismatch"))
	}
	return nil
}

// currentETag re-renders the aliased entity to get the tag a GET would return.
//
// A missing entity yields "" — no tag can match it, so an If-Match write
// against a deleted entity fails the precondition rather than proceeding into
// the create path.
func (b *caldavBackend) currentETag(
	ctx context.Context, collection string, m *caldavMapper, alias caldavalias.Alias,
) (string, error) {
	src := feedEntitySource{app: b.app}
	e, found, err := src.getEntity(ctx, m.cfg.EntityType, alias.EntityID)
	if err != nil {
		return "", err
	}
	if !found {
		return "", nil
	}
	obj, err := b.renderObject(ctx, collection, m, e, alias.Href, alias.UID)
	if err != nil {
		return "", err
	}
	return obj.ETag, nil
}

// createFromTodo creates a new entity for a client-originated to-do.
//
// The client chose the href and UID (Apple uses a bare UUID for both), neither
// of which can be a rela entity id, so the alias is what links them. Written
// AFTER the entity exists: an alias pointing at a nonexistent entity would make
// every later request 404.
func (b *caldavBackend) createFromTodo(
	ctx context.Context, collection, href string, m *caldavMapper, in inboundTodo,
) (*caldav.CalendarObject, error) {
	patch, err := m.createPatch(in)
	if err != nil {
		return nil, err
	}
	newEntity := &entitypkg.Entity{Type: m.cfg.EntityType, Properties: patch.Properties}
	if patch.Content != nil {
		newEntity.Content = *patch.Content
	}
	created, err := b.app.entityManager.CreateEntity(ctx, newEntity, entitypkg.CreateOptions{})
	if err != nil {
		// A DENIED create has no stored entity to serve back, so the
		// accept-and-show-the-truth answer used for updates is unavailable —
		// there is no truth yet. 404 is the honest reply: nothing exists at this
		// href, which is exactly what the client should conclude. Unlike a
		// refused UPDATE, this leaves no divergent local copy to reconcile,
		// because the client's item never became a server resource.
		var denied *acl.ForbiddenError
		if errors.As(err, &denied) {
			// Same 404, same message as every other "nothing here" — see
			// notFoundHere. A create-denial that named itself would let a caller
			// separate "create refused" from "resource exists but hidden".
			return nil, notFoundHere()
		}
		return nil, caldavWriteError(err)
	}

	obj, err := b.renderObject(ctx, collection, m, created.Entity, href, in.Todo.UID)
	if err != nil {
		return nil, err
	}
	if err := b.app.caldavAliases.Put(ctx, caldavalias.Alias{
		Principal:  aliasPrincipal(ctx),
		Collection: collection, Href: href, UID: in.Todo.UID,
		EntityID: created.Entity.ID,
	}); err != nil {
		return nil, fmt.Errorf("caldav: record alias: %w", err)
	}
	return obj, nil
}

// updateFromTodo applies a client edit to an existing entity.
func (b *caldavBackend) updateFromTodo(
	ctx context.Context, collection, href string, m *caldavMapper, in inboundTodo, entityID string,
) (*caldav.CalendarObject, error) {
	patch, err := m.patchFor(in)
	if err != nil {
		return nil, err
	}
	res, err := b.app.entityManager.PatchEntity(ctx, entityID, patch)
	if errors.Is(err, entitymanager.ErrEntityNotFound) || errors.Is(err, store.ErrNotFound) {
		// The alias points at an entity the write could not find, which USUALLY
		// means it was deleted in rela (the SPA, the CLI, a git pull) while this
		// client still held the resource — and the deletion should win.
		//
		// But "not found" here is not a trustworthy signal on its own:
		// entitymanager collapses ANY GetEntity failure into ErrEntityNotFound
		// (manager.go, "structural, not textual"), so malformed frontmatter from
		// a bad merge, a transient EIO, or a dropped pgx connection all arrive
		// looking like a deletion. Answering those with staleWriteResponse would
		// be unrecoverable: the 404 is deliberately permanent so the client
		// DROPS its local copy, and the entity is still on disk — the user loses
		// the only copy they were looking at, to a transient fault.
		//
		// So confirm the absence positively before acting on it.
		if b.entityIsGone(ctx, m, entityID) {
			return nil, staleWriteResponse()
		}
		return nil, webdav.NewHTTPError(http.StatusServiceUnavailable,
			errors.New("caldav: this to-do is temporarily unreadable; retry later"))
	}
	var denied *acl.ForbiddenError
	if errors.As(err, &denied) {
		return b.refusedWriteResponse(ctx, collection, href, m, in, entityID)
	}
	if err != nil {
		return nil, caldavWriteError(err)
	}

	obj, err := b.renderObject(ctx, collection, m, res.Entity, href, in.Todo.UID)
	if err != nil {
		return nil, err
	}
	// Refresh the stored ETag so the next If-Match compares against what the
	// client now holds.
	if err := b.app.caldavAliases.Put(ctx, caldavalias.Alias{
		Principal:  aliasPrincipal(ctx),
		Collection: collection, Href: href, UID: in.Todo.UID,
		EntityID: res.Entity.ID,
	}); err != nil {
		return nil, fmt.Errorf("caldav: record alias: %w", err)
	}
	// A `read_only:` field the client sent was discarded, so what we stored is
	// NOT what was submitted and RFC 4791 §5.3.4 forbids a strong ETag here.
	// Withholding it is also what makes the client re-read and show the real
	// value, instead of caching its own rejected edit as current.
	if m.droppedReadOnly(in) {
		obj.ETag = ""
	}
	return obj, nil
}

// refusedWriteResponse answers a DENIED write by accepting it and serving the
// entity as it actually stands — unchanged.
//
// # Why a refusal is reported as success
//
// Because the alternative does not work, and we do not control the clients.
// A CalDAV client is software we cannot fix; when it mishandles a refusal the
// user is left with a to-do app showing edits that will never exist, and no
// status code recovers it:
//
//   - 403 — Thunderbird classifies it as an AUTH error, never reaches the code
//     path that discards a pending edit, and keeps the local change forever
//     (it lives in its offline_journal SQLite table, so a restart does not
//     clear it). Verified on the wire and in its source.
//   - 412/409 — reaches the reconciliation prompt, but "Submit my changes
//     anyway" re-PUTs, is refused again, and re-prompts, with no retry counter
//     anywhere in that path. It also fires during BACKGROUND playback, so a
//     permanently-refused item raises a blocking modal on every sync. Tried
//     against a live client and reverted; worse than 403.
//   - 404 — does clear the client's copy, but it is a lie about a resource we
//     are still serving, and the user loses a live to-do from their list.
//
// So the honest options all fail the user. Accepting the write and serving the
// truth back is what makes the client converge: it drops its pending state and
// re-reads, and the user watches their unauthorized edit revert. That was
// verified end to end — an item stuck across restarts cleared instantly.
//
// This is not a workaround invented here. Apple's CalendarServer, the reference
// implementation, does exactly this for VTODO: replaceMissingToDoProperties
// restores organizer/attendee properties a client tried to remove, keeps the
// 2xx, and suppresses the ETag. Server-owned fields are normalized, not refused.
//
// # What is NOT given up
//
// The denial is real. entitymanager already refused the write, the entity is
// untouched, and an audit row records it with the principal, the rule, and the
// attempted op. What changes is only what the CLIENT is told, and the client is
// told the truth in the representation: it reads back the stored values.
//
// The cost is deliberate and worth naming: a caller cannot distinguish "written"
// from "refused" by status alone, so a CalDAV client is not a way to probe
// permissions. Anything that needs an authoritative answer uses the API, which
// still answers 403.
//
// # The ETag MUST be suppressed
//
// RFC 4791 §5.3.4 and RFC 9110 §9.3.4: a server may only return a strong ETag
// on PUT when the stored representation is octet-equal to what was submitted.
// Here it never is — we stored nothing. Returning one would let the client cache
// a tag for content it does not have, and a later If-Match against it would
// succeed on a write we would have refused. Withholding it is also what MAKES
// this work: with no valid tag the client must re-read to learn the state, which
// is precisely the reconciliation we want.
func (b *caldavBackend) refusedWriteResponse(
	ctx context.Context, collection, href string, m *caldavMapper, in inboundTodo, entityID string,
) (*caldav.CalendarObject, error) {
	src := feedEntitySource{app: b.app}
	e, found, err := src.getEntity(ctx, m.cfg.EntityType, entityID)
	if err != nil || !found {
		// No readable entity, so there is nothing truthful to hand back.
		//
		// This answers the SAME 404 a nonexistent href gets, and that is the
		// whole point. getEntity returns !found for BOTH "absent" and "you may
		// not read it" — deliberately indistinguishable (RR-NGMI). Returning 403
		// here would undo that: a 403 would mean "an entity exists at this href
		// and you cannot read it", against 404 for "nothing here", which is a
		// clean existence oracle for any href an attacker can name. Entity
		// existence is the thing the read gate is protecting.
		//
		// The message is byte-identical to the create-denial 404 for the same
		// reason: two distinguishable 404s are still an oracle, just a quieter
		// one.
		return nil, notFoundHere()
	}
	obj, err := b.renderObject(ctx, collection, m, e, href, in.Todo.UID)
	if err != nil {
		return nil, err
	}
	// Stored bytes differ from the submitted ones (we stored none), so no ETag.
	obj.ETag = ""
	return obj, nil
}

// notFoundHere is the ONE answer for "there is nothing you can address at this
// href", whatever the underlying reason.
//
// Four distinct conditions share it deliberately: the resource never existed,
// it exists but the principal may not read it, the collection refuses creates,
// and the entity was deleted after we served it. Distinguishing any of them
// would leak the fact the read gate exists to hide — whether a given entity
// EXISTS (RR-NGMI). The message carries no entity id, no type, and no hint of
// which case fired.
//
// This costs an operator some debuggability, which is the accepted trade: the
// audit log records the denial with the principal, the rule and the attempted
// op, so the answer is available server-side to someone authorized to have it.
func notFoundHere() error {
	return webdav.NewHTTPError(http.StatusNotFound,
		errors.New("caldav: no such to-do"))
}

// entityIsGone re-reads the entity to confirm it is genuinely absent, rather
// than merely unreadable right now.
//
// Only a positive not-found answer justifies the permanent 404, because that
// response tells the client to discard its copy. Anything else — an I/O error,
// a parse failure, a dead connection — is reported as retryable, so a transient
// fault costs a retry instead of the user's data.
// The probe goes to the STORE, not through the ACL read path: getVisible maps
// every store error to (nil,false,nil) on purpose, so that a denied read is
// indistinguishable from a real miss — which is right for the read gate and
// useless here, where telling those apart is the whole question. This asks only
// "does this row exist?", never returning content, so it discloses nothing the
// caller has not already proven it may write to (the alias binds this href to
// this entity, and the write itself is separately authorized).
func (b *caldavBackend) entityIsGone(ctx context.Context, _ *caldavMapper, entityID string) bool {
	_, err := b.app.Services().Store.GetEntity(ctx, entityID)
	return errors.Is(err, store.ErrNotFound)
}

// staleWriteResponse answers a PUT whose alias points at an entity that is gone.
//
// # The alias table IS the tombstone
//
// An alias is a durable record that this server once served this resource. So
// "alias exists, entity does not" is proof the entity was deleted after we
// served it — an inference drawn entirely from server-side state. It therefore
// holds no matter what the client does, unlike a marker property echoed in the
// request body, which RFC 5545 3.8.8.2 lets any client silently drop ("User
// agents ... can ignore them").
//
// It is also indifferent to HOW the entity was deleted, which is what makes it
// work on the filesystem backend. A `rm`, a `git pull`, or an edit while the
// server was stopped fires no event and reaches no hook — fsstore's startup
// scan simply adopts whatever is on disk (see syncEntities), so the deletion
// leaves no trace anywhere else. Asking "is it there now?" needs no event.
//
// # Why the alias is KEPT
//
// Deleting it would discard the very evidence the tombstone rests on: the next
// PUT would find no alias, read as a create, and resurrect the entity. Keeping
// it makes the 404 stable across every retry.
//
// 404 rather than 409 because the condition is permanent. A CalDAV client reads
// 409 as "retry later" and re-sends every sync cycle forever; 404 tells it to
// drop its local copy, which is the outcome that matches the deletion.
func staleWriteResponse() error {
	// Deliberately the same opaque 404 as every other "nothing here": a message
	// naming the deletion would confirm the entity USED to exist, which is the
	// same secret as confirming it exists now.
	return notFoundHere()
}

// DeleteCalendarObject applies the collection's configured delete behavior.
//
// The default is a STATUS TRANSITION, not an entity delete: the client gesture
// is a swipe, rela has no soft-delete, and DeleteEntity cascades to relations —
// so a mis-swipe would destroy a graph node and its edges. An operator opts into
// a real delete with `on_delete: {hard: true}`; with no on_delete configured at
// all the request is refused rather than guessed.
func (b *caldavBackend) DeleteCalendarObject(ctx context.Context, p string) error {
	name, href, ok := b.splitPath(p)
	if !ok || href == "" {
		return webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: not found"))
	}
	m, _, err := b.mapperFor(name)
	if err != nil {
		return err
	}

	entityID, ok := b.entityIDFor(ctx, name, href, m)
	if !ok {
		return webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: not found"))
	}

	patch, hard, configured := m.deletePatch()
	if !configured {
		return webdav.NewHTTPError(http.StatusForbidden,
			errors.New("caldav: this collection does not permit deletion (no on_delete configured)"))
	}

	if hard {
		// The alias is deliberately KEPT: it becomes the tombstone that lets a
		// later PUT from a client that has not synced be refused rather than
		// resurrecting this entity. See staleWriteResponse.
		if _, err := b.app.entityManager.DeleteEntity(ctx, entityID, false); err != nil {
			return caldavWriteError(err)
		}
		return nil
	}

	if _, err := b.app.entityManager.PatchEntity(ctx, entityID, patch); err != nil {
		return caldavWriteError(err)
	}
	// The alias is KEPT, exactly as on the hard-delete path.
	//
	// Dropping it looked right — the resource is gone from the client's view —
	// but it throws away the only record that this href belongs to this entity,
	// and two bugs follow immediately:
	//
	//   - the resource comes BACK under a different href. objectFor falls back
	//     to the derived <type>--<id>@rela.ics form when no alias exists, so if
	//     the entity still matches `where:` after the transition it reappears
	//     with a new identity, which a client reads as delete-plus-create.
	//   - an offline client replaying its cached PUT creates a SECOND entity.
	//     With no alias, and a client-minted UUID that cannot satisfy
	//     splitFeedUID, the write falls through to createFromTodo — the exact
	//     duplication registerCalDAVRoutes refuses to start without an alias
	//     service to prevent.
	//
	// Both were reproduced. Keeping the alias means the href stays bound to the
	// entity, so a replayed PUT is an update of the transitioned entity rather
	// than a create.
	return nil
}

// entityIDFor resolves the entity behind a resource href, via the alias or the
// derived UID form.
// The resolved entity's ACTUAL type is checked against the collection, not the
// type asserted by the href. A collection declares exactly one entity_type, so
// an id that resolves to some other type is not a member of this collection and
// must not be writable or deletable through it — and on the derived-UID branch
// the type in the href is supplied by the CLIENT, so trusting it lets a
// task-shaped href address an entity of another type entirely.
func (b *caldavBackend) entityIDFor(ctx context.Context, collection, href string, m *caldavMapper) (string, bool) {
	id, ok := b.resolveEntityID(ctx, collection, href, m)
	if !ok {
		return "", false
	}
	e, err := b.app.Services().Store.GetEntity(ctx, id)
	if err != nil || e.Type != m.cfg.EntityType {
		return "", false
	}
	return id, true
}

// resolveEntityID maps an href to a candidate entity id, without validating it.
func (b *caldavBackend) resolveEntityID(
	ctx context.Context, collection, href string, m *caldavMapper,
) (string, bool) {
	if alias, ok := b.app.caldavAliases.Lookup(aliasPrincipal(ctx), collection, href); ok {
		return alias.EntityID, true
	}
	uid := strings.TrimSuffix(href, ".ics")
	if t, id, ok := splitFeedUID(uid); ok && t == m.cfg.EntityType {
		return id, true
	}
	return "", false
}

// caldavWriteError maps a write failure onto a CalDAV status.
//
// Most rela write failures are soft by policy (DEC-HWZHA) and never reach here.
//
// The status choice is load-bearing for client behavior, not cosmetic. A
// CalDAV client treats 409 as "retry later" and will re-send the same request
// on every sync cycle, forever, with no user-visible signal — so a PERMANENT
// condition must never be reported as 409 or the client wedges. Only a
// genuinely transient conflict (a unique-constraint collision that a later
// write could win) earns it.
//
// # A denied write stays 403, and 412 was tried and reverted
//
// This is worth writing down because 412 looks like an improvement and is not.
//
// The observed problem: Thunderbird never discards a refused edit. It classifies
// 403 as an auth error (CalDavRequest.sys.mjs: `get authError()` covers 401 and
// 403) and only 409/412 reach `promptOverwrite`, the one path that can drop a
// pending local change. So on 403 the edit stays in its local offline journal —
// SQLite, so it survives a restart — is re-sent every sync, and the user goes on
// seeing a change that will never exist. Mozilla bugs 1968933 / 2048378 / 1220745.
//
// 412 does reach that path, which is why it was tried. It is still worse:
//
//   - The dialog LIES. It reads "Item changed on server … Submitting your
//     changes will overwrite the changes made on the server." Nothing changed on
//     the server; the write was refused. Contention is the only cause the dialog
//     contemplates.
//   - "Submit my changes anyway" LOOPS. It re-PUTs with aIgnoreEtag=true and, for
//     a permanent denial, gets 412 again and re-prompts. There is no retry
//     counter or loop guard in that path. (A real etag race terminates because
//     the retry drops If-Match and then succeeds — which is why the missing guard
//     has never bitten.)
//   - It fires in the BACKGROUND. playbackOfflineItems calls modifyItem with no
//     user interaction and no suppression, so a permanently-refused item raises a
//     blocking modal on every refresh — every 30 minutes, per item, indefinitely.
//
// So the choice is between a silent stale copy (403) and a recurring modal the
// user cannot resolve (412). Silent-but-stable wins: it is recoverable by
// removing and re-adding the calendar, where the modal is not escapable at all.
//
// The real fix is to not refuse the write. A collection that should not accept
// client edits wants `read_only:` (accept the write, discard the field) rather
// than an ACL denial — every write succeeds, Thunderbird re-fetches after any
// 2xx, and no pending state is ever created. See docs/caldav-clients.md.
func caldavWriteError(err error) error {
	if err == nil {
		return nil
	}
	var denied *acl.ForbiddenError
	if errors.As(err, &denied) {
		return webdav.NewHTTPError(http.StatusForbidden, err)
	}
	// The entity is gone. Permanent from the client's perspective: 404 tells it
	// to drop the resource, where 409 would make it retry the same doomed write
	// every cycle.
	if errors.Is(err, entitymanager.ErrEntityNotFound) || errors.Is(err, store.ErrNotFound) {
		return webdav.NewHTTPError(http.StatusNotFound, err)
	}
	return webdav.NewHTTPError(http.StatusConflict, err)
}
