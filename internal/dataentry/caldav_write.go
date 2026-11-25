package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/emersion/go-ical"
	"github.com/emersion/go-webdav"
	"github.com/emersion/go-webdav/caldav"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/caldavalias"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entitypkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
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
	m, cfg, err := b.mapperFor(ctx, name)
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
		if t, id, split := splitFeedUID(in.Todo.UID); split && t == cfg.EntityType {
			entityID = id
		}
	}

	if entityID == "" {
		return b.createFromTodo(ctx, name, href, m, in)
	}
	return b.updateFromTodo(ctx, name, href, m, in, entityID, !hasExisting)
}

// checkPreconditions enforces If-Match / If-None-Match, so a client that has
// been offline cannot overwrite a change made elsewhere (412).
//
// The comparison is against the ETag the resource would be served RIGHT NOW,
// re-rendered here, never a cached copy: a cached tag goes stale on any
// rela-side edit, which either wedges the client on a permanent 412 or lets a
// stale tag overwrite a newer edit. A resource without an alias may still
// exist under its derived href, so a missing alias alone never fails If-Match.
func (b *caldavBackend) checkPreconditions(
	ctx context.Context, opts *caldav.PutCalendarObjectOptions,
	collection, href string, m *caldavMapper, existing caldavalias.Alias, hasExisting bool,
) error {
	if opts == nil {
		return nil
	}
	if opts.IfNoneMatch.IsSet() && hasExisting {
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
	if !hasExisting {
		derivedUID := strings.TrimSuffix(href, ".ics")
		t, id, ok := splitFeedUID(derivedUID)
		if !ok || t != m.cfg.EntityType {
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
// The client chose the href and UID, neither of which can be a rela entity
// id, so the alias links them. It is written AFTER the entity exists. A
// denied create answers notFoundHere, the same 404 as every "nothing here",
// so a caller cannot tell "create refused" from "resource exists but hidden".
// In a dynamic collection the entity is linked through linkToDriver.
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
		var denied *acl.ForbiddenError
		if errors.As(err, &denied) {
			return nil, notFoundHere()
		}
		return nil, caldavWriteError(err)
	}

	if linkErr := b.linkToDriver(ctx, collection, created.Entity.Ref()); linkErr != nil {
		return nil, linkErr
	}

	obj, err := b.renderObject(ctx, collection, m, created.Entity, href, in.Todo.UID)
	if err != nil {
		return nil, err
	}
	if err := b.app.caldavAliases.Put(ctx, caldavalias.Alias{
		Principal:  aliasPrincipal(ctx),
		Collection: collection, Href: href, UID: in.Todo.UID,
		EntityID: created.Entity.Ref().String(),
	}); err != nil {
		return nil, fmt.Errorf("caldav: record alias: %w", err)
	}
	return obj, nil
}

// attachToDriver creates the relation that makes an entity a member of a
// dynamic collection. It is a no-op for static collections and idempotent
// for an entry that is already a member.
//
// With joining set, a single-valued membership leaves the other collection
// (see singleMemberships). Without it, the membership is only re-asserted, so
// a stale edit cannot move the entity back. It has NO side effect on the
// entity; linkToDriver and updateFromTodo decide what a failure undoes.
func (b *caldavBackend) attachToDriver(
	ctx context.Context, collection string, member entitypkg.Ref, joining bool,
) error {
	dyn, driverID, ok := b.resolveDynamic(ctx, collection)
	if !ok {
		return nil
	}
	key := b.membershipKey(dyn, driverID, member)
	var others []entitypkg.RelationKey
	if joining {
		var err error
		if others, err = singleMemberships(ctx, b.app, dyn, key); err != nil {
			return err
		}
	}
	var err error
	if len(others) > 0 {
		_, err = b.app.entityManager.ReplaceRelations(ctx,
			[]entitymanager.RelationCreate{{Key: key}}, others)
	} else {
		_, err = b.app.entityManager.CreateRelation(ctx, key, entitypkg.RelationOptions{})
	}
	if err == nil || errors.Is(err, entitymanager.ErrRelationAlreadyExists) {
		return nil
	}
	return err
}

// singleMemberships returns the member's other membership edges that joining
// the collection key names must remove: those of a relation that allows the
// member one edge (max_outgoing: 1 when the member is the source,
// max_incoming: 1 when it is the target). For any other relation it returns
// none, and joining is additive. Only edges whose endpoints the principal may
// read are returned, as the relations PATCH plans them; the manager then
// authorizes each delete. An edge the caller cannot see stays, and the bound
// refuses the join.
func singleMemberships(
	ctx context.Context, app *App, dyn dataentryconfig.CalDAVDynamicCollection, key entitypkg.RelationKey,
) ([]entitypkg.RelationKey, error) {
	def, ok := app.State().Meta.Relations[dyn.Relation]
	if !ok {
		return nil, nil
	}
	incoming := dyn.Direction.IsIncoming()
	bound := def.MaxOutgoing
	q := store.RelationQuery{From: key.From, Type: key.Type}
	if incoming {
		bound = def.MaxIncoming
		q = store.RelationQuery{To: key.To, Type: key.Type}
	}
	if bound == nil || *bound != 1 {
		return nil, nil
	}
	var edges []*entitypkg.Relation
	for rel, err := range app.Services().Store.ListRelations(ctx, q) {
		if err != nil {
			return nil, fmt.Errorf("caldav: list memberships: %w", err)
		}
		if rel.Identity() == key {
			continue
		}
		if !incoming && rel.FromFace != key.FromFace {
			continue
		}
		edges = append(edges, rel)
	}
	edges, err := app.visibleReader.readableRelations(ctx, edges)
	if err != nil {
		return nil, err
	}
	out := make([]entitypkg.RelationKey, len(edges))
	for i, rel := range edges {
		out[i] = rel.Identity()
	}
	return out, nil
}

// membershipKey is the edge that makes member part of a dynamic collection.
// An outgoing edge's source is the member: a content-scoped one tails at the
// member's face, an identity-scoped one at the implicit face. An incoming
// edge's source is the driver, which a collection names by bare id, so it
// tails at the implicit face; the manager refuses that for a content-scoped
// relation from a faced driver (entitymanager.ErrRelationFaceRequired).
func (b *caldavBackend) membershipKey(
	dyn dataentryconfig.CalDAVDynamicCollection, driverID string, member entitypkg.Ref,
) entitypkg.RelationKey {
	if dyn.Direction.IsIncoming() {
		return entitypkg.RelationKey{
			From: driverID, FromFace: entitypkg.ImplicitFace, Type: dyn.Relation, To: member.ID,
		}
	}
	tail := entitypkg.ImplicitFace
	if metamodel.IsContentScoped(b.app.State().Meta, dyn.Relation) {
		tail = member.Face
	}
	return entitypkg.RelationKey{From: member.ID, FromFace: tail, Type: dyn.Relation, To: driverID}
}

// linkToDriver attaches a NEWLY CREATED entity to its dynamic collection and
// deletes that entity, non-cascading, if the edge cannot be made. Membership
// IS the relation, so an entity without it would be an invisible orphan.
//
// ONLY the create flow may call this. On an edit the entity is arbitrarily old
// and holds the user's data, so the update flow calls attachToDriver directly.
// If the compensating delete itself fails, the error says so.
func (b *caldavBackend) linkToDriver(ctx context.Context, collection string, member entitypkg.Ref) error {
	entityID := member.ID
	err := b.attachToDriver(ctx, collection, member, true)
	if err == nil {
		return nil
	}
	if _, delErr := b.app.entityManager.DeleteEntity(ctx, entityID, false); delErr != nil {
		return fmt.Errorf(
			"caldav: linking %s to collection %q failed (%w), and removing the now-orphaned entity "+
				"also failed (%v) — it exists in no collection", entityID, collection, err, delErr)
	}
	return caldavWriteError(err)
}

// updateFromTodo applies a client edit to an existing entity.
//
// A not-found from the manager may be a transient fault, so absence is
// confirmed through entityIsGone before the permanent staleWriteResponse; an
// unreadable row gets the same answer, so a 503 cannot confirm it exists.
// Writing into a dynamic collection adds a membership through attachToDriver,
// never linkToDriver, whose compensating delete would destroy user data.
func (b *caldavBackend) updateFromTodo(
	ctx context.Context, collection, href string, m *caldavMapper, in inboundTodo, entityID string,
	joining bool,
) (*caldav.CalendarObject, error) {
	patch, err := m.patchFor(in)
	if err != nil {
		return nil, err
	}
	addr, ambiguous, err := b.writeAddress(ctx, m, entityID)
	if err != nil {
		return nil, caldavWriteError(err)
	}
	if ambiguous {
		return b.refusedWriteResponse(ctx, collection, href, m, in, entityID)
	}
	res, err := b.app.entityManager.PatchEntity(ctx, addr, patch)
	if errors.Is(err, entitymanager.ErrEntityNotFound) || errors.Is(err, store.ErrNotFound) {
		if b.entityIsGone(ctx, m, entityID) || entityIsHidden(ctx, b.app, entityID) {
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

	if linkErr := b.attachToDriver(ctx, collection, res.Entity.Ref(), joining); linkErr != nil {
		var linkDenied *acl.ForbiddenError
		_, overBound := errors.AsType[*entitymanager.CardinalityError](linkErr)
		if errors.As(linkErr, &linkDenied) || overBound {
			return b.refusedWriteResponse(ctx, collection, href, m, in, entityID)
		}
		return nil, caldavWriteError(linkErr)
	}

	obj, err := b.renderObject(ctx, collection, m, res.Entity, href, in.Todo.UID)
	if err != nil {
		return nil, err
	}
	if err := b.app.caldavAliases.Put(ctx, caldavalias.Alias{
		Principal:  aliasPrincipal(ctx),
		Collection: collection, Href: href, UID: in.Todo.UID,
		EntityID: res.Entity.Ref().String(),
	}); err != nil {
		return nil, fmt.Errorf("caldav: record alias: %w", err)
	}
	if m.droppedReadOnly(in) {
		obj.ETag = ""
	}
	return obj, nil
}

// refusedWriteResponse answers a DENIED write by accepting it and serving the
// entity as it actually stands, unchanged.
//
// CalDAV clients mishandle every refusal status (403 keeps the edit forever,
// 412/409 loop on a modal, 404 drops a live to-do), so serving the truth back
// is what makes them converge. The denial is still real and audited. The ETag
// MUST be suppressed (RFC 4791 §5.3.4): nothing was stored. With no readable
// entity it answers notFoundHere, never 403, which would be an existence
// oracle. See docs/caldav-clients.md.
func (b *caldavBackend) refusedWriteResponse(
	ctx context.Context, collection, href string, m *caldavMapper, in inboundTodo, entityID string,
) (*caldav.CalendarObject, error) {
	src := feedEntitySource{app: b.app}
	e, found, err := src.getEntity(ctx, m.cfg.EntityType, entityID)
	if err != nil || !found {
		return nil, notFoundHere()
	}
	obj, err := b.renderObject(ctx, collection, m, e, href, in.Todo.UID)
	if err != nil {
		return nil, err
	}
	obj.ETag = ""
	return obj, nil
}

// notFoundHere is the ONE answer for "there is nothing you can address at this
// href": the resource never existed, is unreadable, the collection refuses
// creates, or the entity was deleted after we served it. Telling these apart
// would leak whether an entity EXISTS. The message carries no id, no type and
// no hint of which case fired; the audit log has the details.
func notFoundHere() error {
	return webdav.NewHTTPError(http.StatusNotFound,
		errors.New("caldav: no such to-do"))
}

// entityIsGone reports whether the entity is positively absent, not merely
// unreadable right now. Only that justifies the permanent 404; any other
// failure is retryable. The probe goes to the STORE, not the ACL read path,
// because the resolver maps every error to a miss. It returns no content, and
// the caller has already proven write access to this href. A faced address is
// gone when its face is not stored; a bare id when no face is stored.
func (b *caldavBackend) entityIsGone(ctx context.Context, _ *caldavMapper, addr string) bool {
	parsed, err := entitypkg.ParseAddress(addr)
	if err != nil {
		return false
	}
	_, faces, err := loadStoredFaces(ctx, b.app.Services().Store, parsed.ID())
	if err != nil {
		return false
	}
	if named, ok := parsed.Named(); ok {
		return !slices.Contains(faces, named.Face)
	}
	return len(faces) == 0
}

// entityIsHidden reports whether the entity at addr is stored but the read
// gate refuses the caller every face of it. It decides from the stored faces
// and the ACL verdicts only, and any error makes it false: a failed read must
// reach the retryable 503, not the permanent 404 that makes the client drop
// its copy.
func entityIsHidden(ctx context.Context, app *App, addr string) bool {
	gate, ok := app.acl.(interface {
		PermitsReadFace(ctx context.Context, entityType, entityID string, face entitypkg.Face) (bool, error)
	})
	if !ok {
		return false
	}
	parsed, err := entitypkg.ParseAddress(addr)
	if err != nil {
		return false
	}
	typ, faces, err := loadStoredFaces(ctx, app.Services().Store, parsed.ID())
	if err != nil || len(faces) == 0 {
		return false
	}
	if named, isNamed := parsed.Named(); isNamed {
		if !slices.Contains(faces, named.Face) {
			return false
		}
		faces = []entitypkg.Face{named.Face}
	}
	for _, f := range faces {
		readable, gErr := gate.PermitsReadFace(ctx, typ, parsed.ID(), f)
		if gErr != nil || readable {
			return false
		}
	}
	return true
}

// staleWriteResponse answers a PUT whose alias points at an entity that is gone.
//
// The alias table IS the tombstone: an alias without an entity proves a
// deletion, however it happened. The alias is KEPT so the next PUT is not read
// as a create that resurrects the entity. It answers 404 rather than 409 since
// the condition is permanent, with the same opaque body as every other miss.
func staleWriteResponse() error {
	return notFoundHere()
}

// unlinkFromDriver removes the membership a DELETE against a dynamic collection
// addresses, and reports whether the entity itself was also disposed of.
// handled=false means the collection is static, so the caller applies
// `on_delete:` to the entity.
//
// Removing the LAST membership is decided by [dataentryconfig.CalDAVUnlinkPolicy].
// The edge is removed FIRST, so a failing on_delete still leaves it gone.
func (b *caldavBackend) unlinkFromDriver(
	ctx context.Context, collection, addr string,
) (handled, entityDisposed bool, err error) {
	dyn, driverID, ok := b.resolveDynamic(ctx, collection)
	if !ok {
		return false, false, nil
	}
	entityID := addressID(addr)
	member := entitypkg.Ref{ID: entityID, Face: entitypkg.ImplicitFace}
	if parsed, perr := entitypkg.ParseAddress(addr); perr == nil {
		if named, ok := parsed.Named(); ok {
			member = named
		}
	}

	last, err := b.isLastMembership(ctx, dyn, entityID)
	if err != nil {
		return false, false, err
	}
	key := b.membershipKey(dyn, driverID, member)
	if delErr := b.app.entityManager.DeleteRelation(ctx, key); delErr != nil {
		if errors.Is(delErr, store.ErrNotFound) {
			return unlinkMissing(ctx, b.app, dyn, entityID)
		}
		return false, false, caldavWriteError(delErr)
	}
	if !last || !b.disposeOnLastUnlink(dyn) {
		return true, false, nil
	}

	m, _, mapErr := b.mapperFor(ctx, collection)
	if mapErr != nil {
		return true, false, mapErr
	}
	patch, hard, configured := m.deletePatch()
	if !configured {
		return true, false, nil
	}
	if hard {
		if e := b.deleteAddressed(ctx, addr); e != nil {
			return true, false, caldavWriteError(e)
		}
		return true, true, nil
	}
	if e := b.patchEntity(ctx, m, addr, patch); e != nil {
		return true, false, e
	}
	return true, true, nil
}

// unlinkMissing answers a DELETE whose membership edge is already gone. That
// is the second half of a client move into a single-valued membership's
// other collection: the PUT there replaced this edge (attachToDriver). The
// DELETE asked for exactly that state, so it succeeds, and it does not count
// as removing the entity's last membership.
//
// It succeeds only when the principal may read the entity and it has another
// readable membership of the same relation, which is what a move leaves.
// Anything else is the opaque 404 of a resource that is not here: answering
// success for any id would tell a caller which ids exist.
func unlinkMissing(
	ctx context.Context, app *App, dyn dataentryconfig.CalDAVDynamicCollection, entityID string,
) (handled, entityDisposed bool, err error) {
	types, err := app.visibleReader.readableTypes(ctx, []string{entityID})
	if err != nil {
		return false, false, err
	}
	if types[entityID] == "" {
		return false, false, notFoundHere()
	}
	dir := store.DirectionOutgoing
	if dyn.Direction.IsIncoming() {
		dir = store.DirectionIncoming
	}
	rels, err := listRelationsCtx(ctx, app.Services().Store,
		store.RelationQuery{EntityID: entityID, Type: dyn.Relation, Direction: dir})
	if err != nil {
		return false, false, fmt.Errorf("caldav: list memberships: %w", err)
	}
	if rels, err = app.visibleReader.readableRelations(ctx, rels); err != nil {
		return false, false, err
	}
	if len(rels) == 0 {
		return false, false, notFoundHere()
	}
	return true, false, nil
}

// deleteAddressed hard-deletes what a resource's address names: the face it
// names, or the whole entity for a faceless type's bare id. A faced to-do is
// served under its face's address (see [feedUID]), so a calendar delete
// removes the face the client was shown, never its other faces.
func (b *caldavBackend) deleteAddressed(ctx context.Context, addr string) error {
	parsed, err := entitypkg.ParseAddress(addr)
	if err != nil {
		return err
	}
	if named, ok := parsed.Named(); ok {
		_, err = b.app.entityManager.DeleteEntityFace(ctx, named.ID, named.Face, false)
		return err
	}
	_, err = b.app.entityManager.DeleteEntity(ctx, parsed.ID(), false)
	return err
}

// addressID is the entity id of a resource address, which carries a face
// for a faced to-do. An address that does not parse is returned unchanged,
// so the read it feeds answers its own miss.
func addressID(addr string) string {
	parsed, err := entitypkg.ParseAddress(addr)
	if err != nil {
		return addr
	}
	return parsed.ID()
}

// isLastMembership reports whether the entity's only remaining edge of this
// relation type is the one about to be removed.
func (b *caldavBackend) isLastMembership(
	ctx context.Context, dyn dataentryconfig.CalDAVDynamicCollection, entityID string,
) (bool, error) {
	dir := store.DirectionOutgoing
	if dyn.Direction.IsIncoming() {
		dir = store.DirectionIncoming
	}
	rels, err := listRelationsCtx(ctx, b.app.Services().Store,
		store.RelationQuery{EntityID: entityID, Type: dyn.Relation, Direction: dir})
	if err != nil {
		return false, fmt.Errorf("caldav: count memberships: %w", err)
	}
	return len(rels) <= 1, nil
}

// disposeOnLastUnlink resolves the unlink policy for a pattern.
//
// Under `auto` the relation's declared cardinality decides: a mandatory
// membership (`min_outgoing >= 1`) means an orphan contradicts the operator's
// own schema, so the entity is disposed of via on_delete:.
func (b *caldavBackend) disposeOnLastUnlink(dyn dataentryconfig.CalDAVDynamicCollection) bool {
	switch dyn.OnUnlink.OrDefault() {
	case dataentryconfig.CalDAVUnlinkKeep:
		return false
	case dataentryconfig.CalDAVUnlinkDelete:
		return true
	default:
		def, ok := b.app.State().Meta.Relations[dyn.Relation]
		if !ok {
			return false
		}
		required := def.MinOutgoing
		return required != nil && *required >= 1
	}
}

// DeleteCalendarObject applies the collection's configured delete behavior.
//
// The default is a STATUS TRANSITION, not an entity delete. An operator opts
// into a real delete with `on_delete: {hard: true}`; with no on_delete the
// request is refused. A DELETE in a dynamic collection removes only the
// membership. The alias is KEPT on every path, so a replayed PUT updates the
// entity instead of creating a duplicate or reappearing under a new href.
func (b *caldavBackend) DeleteCalendarObject(ctx context.Context, p string) error {
	name, href, ok := b.splitPath(p)
	if !ok || href == "" {
		return webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: not found"))
	}
	m, _, err := b.mapperFor(ctx, name)
	if err != nil {
		return err
	}

	entityID, ok := b.entityIDFor(ctx, name, href, m)
	if !ok {
		return webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: not found"))
	}

	unlinked, entityGone, err := b.unlinkFromDriver(ctx, name, entityID)
	if err != nil {
		return err
	}
	if unlinked && !entityGone {
		return nil
	}
	if unlinked && entityGone {
		return nil
	}

	patch, hard, configured := m.deletePatch()
	if !configured {
		return webdav.NewHTTPError(http.StatusForbidden,
			errors.New("caldav: this collection does not permit deletion (no on_delete configured)"))
	}

	if hard {
		if err := b.deleteAddressed(ctx, entityID); err != nil {
			return caldavWriteError(err)
		}
		return nil
	}

	return b.patchEntity(ctx, m, entityID, patch)
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
	if typ := storedTypeOf(ctx, b.app.Services().Store, addressID(id)); typ == "" || typ != m.cfg.EntityType {
		return "", false
	}
	return id, true
}

// writeAddress resolves entityID to the face a CalDAV write edits, by the
// rule every write follows ([visibility.Resolver.WriteTarget]). A faced
// to-do's href and UID carry its face ([feedUID]), so the client names the
// face it was shown; a bare id names the implicit face of a faceless type.
// ambiguous reports a bare id on a faced type, which a legacy href can still
// carry. A miss returns entityID unchanged, so the write answers the
// not-found its caller already handles.
func (b *caldavBackend) writeAddress(
	ctx context.Context, m *caldavMapper, entityID string,
) (addr string, ambiguous bool, err error) {
	parsed, err := entitypkg.ParseAddress(entityID)
	if err != nil {
		return entityID, false, nil //nolint:nilerr // an unparseable id is a miss, answered by the write
	}
	ref, ok, err := b.app.visibleReader.resolver.WriteTarget(
		ctx, worldFromContext(ctx).visibility(), m.cfg.EntityType, parsed)
	if _, isAmb := errors.AsType[*visibility.AmbiguousAddressError](err); isAmb {
		return "", true, nil
	}
	if err != nil {
		return "", false, err
	}
	if !ok {
		return entityID, false, nil
	}
	return ref.String(), false, nil
}

// patchEntity applies an on_delete patch to the face entityID resolves to. An
// entity with no single face to write is refused with 403, like any other
// permanent refusal (see [caldavWriteError]).
func (b *caldavBackend) patchEntity(
	ctx context.Context, m *caldavMapper, entityID string, patch entitypkg.Patch,
) error {
	addr, ambiguous, err := b.writeAddress(ctx, m, entityID)
	if err != nil {
		return caldavWriteError(err)
	}
	if ambiguous {
		return webdav.NewHTTPError(http.StatusForbidden,
			errors.New("caldav: this to-do has several faces; edit it in rela"))
	}
	if _, err := b.app.entityManager.PatchEntity(ctx, addr, patch); err != nil {
		return caldavWriteError(err)
	}
	return nil
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
// A client retries 409 forever, so only a transient conflict earns it; a
// permanent condition such as a bound the write would exceed gets the deny
// status. A denied write stays 403: 412 was tried and reverted because it
// loops on a modal in Thunderbird. A missing entity gets the opaque body of
// an unknown href, since the manager's text would separate absent from
// hidden. See docs/caldav-clients.md.
func caldavWriteError(err error) error {
	if err == nil {
		return nil
	}
	var denied *acl.ForbiddenError
	if errors.As(err, &denied) {
		return webdav.NewHTTPError(http.StatusForbidden, err)
	}
	if _, overBound := errors.AsType[*entitymanager.CardinalityError](err); overBound {
		return webdav.NewHTTPError(http.StatusForbidden, err)
	}
	if errors.Is(err, entitymanager.ErrEntityNotFound) || errors.Is(err, store.ErrNotFound) {
		return webdav.NewHTTPError(http.StatusNotFound, errors.New("caldav: not found"))
	}
	return webdav.NewHTTPError(http.StatusConflict, err)
}
