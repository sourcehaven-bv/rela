package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/conflict"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// entityMutator is the write surface the data-entry write handlers call. See
// the internal/entitymanager package doc for the consumer-side rule this
// follows (TKT-IVSJV6).
//
// Seven of the manager's nine write methods. PatchEntity is absent FROM THIS
// HANDLER because a form save renders every field and therefore legitimately
// owns the whole record — the UpdateEntity case (see the PatchEntity rule in
// CLAUDE.md). That is a fact about form saves, not about data-entry writes
// generally: the CalDAV writer patches, and holds its own wider surface
// through App. RenameEntity is absent because the SPA has no rename
// affordance; renames are a CLI and MCP operation.
//
// ValidateCreate is here and nowhere else in the tree: it backs the form's
// dry-run preview, which is advisory only — the real CreateEntity below stays
// the sole authorization and audit point.
type entityMutator interface {
	CreateEntity(
		ctx context.Context, e *entityPkg.Entity, opts entityPkg.CreateOptions,
	) (*entityPkg.CreateResult, error)
	ValidateCreate(
		ctx context.Context, e *entityPkg.Entity, opts entityPkg.CreateOptions,
	) (*entityPkg.Entity, []entityPkg.Warning, error)
	UpdateEntity(ctx context.Context, e *entityPkg.Entity) (*entityPkg.UpdateResult, error)
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entityPkg.DeleteResult, error)
	// DeleteEntityFace removes ONE non-bare content state — what a DELETE
	// addressed to `ID@face` means. See entitymanager.Manager.DeleteEntityFace.
	DeleteEntityFace(ctx context.Context, id string, face entityPkg.Face, cascade bool) (*entityPkg.DeleteResult, error)
	CreateRelation(
		ctx context.Context, key entityPkg.RelationKey, opts entityPkg.RelationOptions,
	) (*entityPkg.Relation, error)
	UpdateRelation(
		ctx context.Context, key entityPkg.RelationKey, opts entityPkg.RelationOptions,
	) (*entityPkg.Relation, error)
	// DeleteRelation removes the edge key names, tail included: what a
	// relation removal addressed to `ID@face` means for a `scope: content`
	// type. The zero tail is the implicit-tail edge, so this also covers
	// every identity-scoped and faceless removal.
	DeleteRelation(ctx context.Context, key entityPkg.RelationKey) error
	// ReplaceRelations creates and deletes edges in one transaction
	// (TKT-65LVAK). See entitymanager.Manager.ReplaceRelations.
	ReplaceRelations(
		ctx context.Context, creates []entitymanager.RelationCreate, removes []entityPkg.RelationKey,
	) ([]*entityPkg.Relation, error)
	// ChecksRelation reports whether a write-enforced relation rule reads
	// relType. See entitymanager.ChecksRelation.
	ChecksRelation(relType string) bool

	// PatchEntity is how the webhook pipeline writes: it names only the
	// properties a hook actually sets, so a property the hook does not mention
	// survives — including one redacted from the read that found the entity.
	// A read-modify-write through UpdateEntity would carry the redacted copy
	// back over the stored one. See the PatchEntity rule in CLAUDE.md.
	PatchEntity(
		ctx context.Context, id string, patch entityPkg.Patch,
	) (*entityPkg.UpdateResult, error)

	// AuthorizeUpdate is the manager's own update check, read gate first,
	// X	// through the grant it returns. See entitymanager.Manager.AuthorizeUpdate.
	AuthorizeUpdate(ctx context.Context, row *entityPkg.Entity) (entitymanager.UpdateGrant, error)

	// PatchGranted is PatchEntity for the row grant names, without asking
	// the ACL again. See entitymanager.UpdateGrant.PatchEntity.
	PatchGranted(
		ctx context.Context, grant entitymanager.UpdateGrant, id string, patch entityPkg.Patch,
	) (*entityPkg.UpdateResult, error)
	// PatchGrantedWithRelations is PatchGranted with a replace of edges at
	// the row, written in the same transaction. See
	// entitymanager.PatchWithRelations.
	PatchGrantedWithRelations(
		ctx context.Context, grant entitymanager.UpdateGrant, id string, patch entityPkg.Patch,
		creates []entitymanager.RelationCreate, removes []entityPkg.RelationKey,
	) (*entityPkg.UpdateResult, error)
}

// grantedManager is the production [entityMutator]: the manager, plus the
// granted write as a method, so tests can wrap it like any other write.
type grantedManager struct{ *entitymanager.Manager }

// PatchGranted writes through the grant. The grant names its own manager,
// which in production is the embedded one.
func (grantedManager) PatchGranted(
	ctx context.Context, grant entitymanager.UpdateGrant, id string, patch entityPkg.Patch,
) (*entityPkg.UpdateResult, error) {
	return grant.PatchEntity(ctx, id, patch)
}

// PatchGrantedWithRelations writes through the grant with
// entitymanager.PatchWithRelations.
func (grantedManager) PatchGrantedWithRelations(
	ctx context.Context, grant entitymanager.UpdateGrant, id string, patch entityPkg.Patch,
	creates []entitymanager.RelationCreate, removes []entityPkg.RelationKey,
) (*entityPkg.UpdateResult, error) {
	res, _, err := entitymanager.PatchWithRelations(ctx, grant, id, patch, creates, removes)
	return res, err
}

// patchJoined writes patch through grant, with the relation writes of joined
// in the same transaction when joined is not nil, and adds joined's warnings
// to the result.
//
// A function, not a method, to keep writeHandler under its plimsoll line.
func patchJoined(
	ctx context.Context, h *writeHandler, grant entitymanager.UpdateGrant, id string, patch entityPkg.Patch,
	joined *joinedRelations,
) (*entityPkg.UpdateResult, error) {
	if joined == nil {
		return h.manager.PatchGranted(ctx, grant, id, patch)
	}
	res, err := h.manager.PatchGrantedWithRelations(ctx, grant, id, patch, joined.creates, joined.removes)
	if err != nil {
		return nil, err
	}
	if res != nil {
		res.Warnings = append(res.Warnings, joined.warnings...)
	}
	return res, nil
}

// writeUpdateError answers a failed entity patch of a PATCH: a refusal of
// the joined relation writes as a relation error, a lost precondition race
// as that, and anything else as a patch error.
//
// A function, not a method, to keep writeHandler under its plimsoll line.
func writeUpdateError(
	w http.ResponseWriter, r *http.Request, h *writeHandler, err error, joined *joinedRelations,
	pre *v1.Preconditions, typeName string, ref entityPkg.Ref, plural string,
) {
	if relErr := joined.relationError(err); relErr != nil {
		h.writeRelationsApplyError(w, r, relErr)
		return
	}
	if pre != nil && h.writeLostRace(w, r, err, pre, typeName, ref, plural) {
		return
	}
	writePatchError(w, r, err)
}

// ChecksRelation reports whether a write-enforced relation rule reads
// relType.
func (g grantedManager) ChecksRelation(relType string) bool {
	return entitymanager.ChecksRelation(g.Manager, relType)
}

// writeHandler owns the data-entry write nucleus: the entity/relation CRUD
// endpoints (create/dry-run-create/update/delete entity, create/update/delete
// relation), clone, conflict-resolve, the modern relations reconciler they
// share, and the Lua action surface (interactive actions + webhook dispatch).
// Extracted from App (TKT-R68TV8 M5.4) to shrink the god object.
//
// This is a PURE STRUCTURAL extraction: the handlers move verbatim and the
// concurrency model is untouched.
//
// Collaborator shape mirrors attachmentHandler (the other write-path handler):
// stable services are held by value (store/manager/reader/serializer/
// affordances), swappable-in-test collaborators are closures over App (schema/
// acl/audit), and the shared helpers used by BOTH the read and write paths are
// passed as closures (denyAfford, computeETag, planEdges)
// so the two paths cannot drift (uniform-404 read gate, affordance-denial
// audit, one ETag definition).
//
// There is no handler-level write lock (TKT-WE0S2K). Correctness under
// concurrent writers comes from below: store-level compare-and-swap, the
// manager's store.Tx around its check-then-write steps, and keyed locks in
// the attachment service. A process-wide mutex could not provide it anyway
// once several rela-server processes share one database.
type writeHandler struct {
	schema func() *Schema
	// rollups adds the roll-up properties (TKT-6I2OPG) to a written row's
	// response, as the GET serves them.
	rollups rollupSource
	store   store.Store
	manager entityMutator
	// softDeletes backs the Undo toast; nil when the store cannot
	// soft-delete. See softDeleter.
	softDeletes softDeleter
	// ifMatchDeletes runs a DELETE that carries If-Match, so the version
	// compare and the delete are one transaction. See ifMatchDeleter.
	ifMatchDeletes ifMatchDeleter
	reader         entityReader
	serializer     entitySerializer
	affordances    affordanceService
	acl            func() acl.ACL
	audit          func() audit.Audit

	// The Lua action surface (interactive actions + webhook dispatch). All
	// three are live closures over App: the engine so fixtures that build App
	// piecemeal see late-set fields, luaDeps because the bundle is derived
	// per call from swappable collaborators, fullScriptDetail because the
	// security layer is wired after construction (SetSecurityConfig).
	engine           func() *script.Engine
	luaDeps          func() lua.WriteDeps
	fullScriptDetail func(r *http.Request) bool
	// versionTags is the tag writer an action script gets (TKT-VO6VG9).
	// Kept out of luaDeps, which document renders, export_render and the
	// webhook share: a tag is a deliberate act, so only an action the caller
	// invoked may write one. Nil, or returning nil: the tag bindings are
	// absent.
	versionTags func() lua.VersionTagWriter
	// oauth is what rela.oauth reads tokens through in an action script
	// (TKT-01KZSO), kept out of luaDeps for the same reason as versionTags.
	// Nil, or returning nil: rela.oauth reports not_configured.
	oauth func() lua.OAuthTokens
	// attachRender backs rela.attach_render in an action script
	// (TKT-1TS55R), kept out of luaDeps for the same reason as versionTags:
	// a document render must not render and attach another. Nil, or
	// returning nil: the binding is absent.
	attachRender func() lua.AttachRenderer
	// attachFile backs rela.attach_file in an action script (TKT-36DHKH8),
	// kept out of luaDeps for the same reason as attachRender. Nil, or
	// returning nil: the binding is absent.
	attachFile func() lua.AttachFiler
	// records backs rela.append_records in an action script (TKT-RK810V7),
	// kept out of luaDeps for the same reason as attachRender. Nil, or
	// returning nil: the binding is absent.
	records func() lua.RecordAppender
	// elevatedUpdates backs admin.update_entity for an action that declares
	// allow_acl_bypass: write (TKT-JA5SABG), kept out of luaDeps for the same
	// reason as versionTags. Nil: such an action fails with 500.
	elevatedUpdates elevatedUpdaterFactory
	// llmJobs queues `apply: auto` runs of `llm:` actions (TKT-WQ925LL).
	// Nil: such a run answers 503.
	llmJobs LLMActionQueue

	// visible is the read path's resolver. Every addressed write resolves its
	// row through it first, so a row the caller may not read, at any face, is
	// the same 404 on a write as on a GET.
	visible visibleReader

	// Shared App helpers (also used by the read path — stay on App).
	denyAfford func(
		ctx context.Context, w http.ResponseWriter, target *entityPkg.Entity, denial AffordanceDenialError,
	)
	computeETag func(ctx context.Context, e *entityPkg.Entity) string
	// faceEdges is the App's gated face-edge read: the outgoing edges of the
	// face an entity IS, with the neighbor ids the caller may see. The write
	// path answers with the row it wrote, so it owes the row's own edges, not
	// the bare id's union of every face's.
	faceEdges faceEdgeReader
	// readVisible is [visibleReader.addressRef]: the GET's gated read,
	// for re-reading a row after a write whose outcome may have hidden it.
	readVisible func(ctx context.Context, typeName string, ref entityPkg.Ref) (*entityPkg.Entity, bool, error)
	// planEdges is edgeReader.plan: the edge writes one relation wrapper
	// asks for, matched against the edges the caller can see.
	planEdges edgePlanner

	// paths contains caller-supplied conflict-file paths to the project
	// root (conflict-resolve is the one file-level write in the nucleus).
	paths *project.Context

	// provision implements unmatched_principal: provision (TKT-ANUJDS). Called
	// at the top of each write handler via withProvision; it lazily creates a
	// stub user entity for an unmatched verified principal and returns a ctx
	// re-stamped to it (with a rebuilt ACL request + read gate).
	// The handler adopts the returned ctx. A no-op on every non-provision write.
	provision func(context.Context) context.Context
}

// withProvision runs the provision seam and returns the request the handler
// must use downstream: re-stamped to a freshly-provisioned entity when
// unmatched_principal: provision fired, else r unchanged.
//
// It returns the *http.Request, not a context, because the read gate and the
// serializer read r.Context(); a bare context would leave them on the stale
// principal and redact the new entity (RR-VI9XMY). The contextcheck findings
// this causes are excluded in .golangci.yml and pinned by
// TestProvisionSeam_EveryWriteHandlerUsesWithProvision.
func (h *writeHandler) withProvision(r *http.Request) *http.Request {
	if h.provision != nil {
		return r.WithContext(h.provision(r.Context()))
	}
	return r
}

// createFace picks the face a create body names, from `face` or `world`.
//
// A world names the face a create from it lands in (`worlds.<name>.create`).
// It rides the body, not `?world=`, which attachWorld refuses on every write.
// The two spellings are exclusive rather than ranked: they can disagree, and
// silently honoring either would write a row the caller did not ask for.
func (h *writeHandler) createFace(
	w http.ResponseWriter, r *http.Request, face, world, typeName string,
	def *metamodel.EntityDef,
) (string, bool) {
	world = strings.TrimSpace(world)
	if world == "" {
		return face, true
	}
	if face != "" {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
			"name either `face` or `world`, not both", "/world")
		return "", false
	}
	return h.createFaceForWorld(w, r, world, typeName, def)
}

// createFaceForWorld resolves a world named in a create body to the face that
// world creates into (`worlds.<name>.create`).
//
// It answers 422 naming the schema key to fix for an undeclared world and for
// a declared world with no `create:`. The second is not silent, because the
// manager's `face_required` would not name the world. The default world
// declares no `create:`, so a create from it names no face.
func (h *writeHandler) createFaceForWorld(
	w http.ResponseWriter, r *http.Request, world, typeName string, def *metamodel.EntityDef,
) (string, bool) {
	if len(def.Faces) == 0 {
		return "", true
	}
	if world == metamodel.DefaultWorldName {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
			fmt.Sprintf("the default world names no face, so %s must be created "+
				"from a world declaring `create:`, or name a `face` directly",
				typeName), "/world")
		return "", false
	}
	wdef, declared := h.schema().Meta.Worlds[world]
	if !declared {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "unknown_world",
			fmt.Sprintf("no world named %q is declared", world), "/world")
		return "", false
	}
	if wdef.Create == "" {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed",
			fmt.Sprintf("world %q declares no `create:` face, so a create cannot be "+
				"issued from it", world), "/world")
		return "", false
	}
	return wdef.Create, true
}

func parseCreateOpts(
	w http.ResponseWriter, r *http.Request, def *metamodel.EntityDef, id, prefix, rawFace string,
) (entityPkg.CreateOptions, bool) {
	opts := entityPkg.CreateOptions{
		ID:     strings.TrimSpace(id),
		Prefix: strings.TrimSpace(prefix),
	}

	if raw := strings.TrimSpace(rawFace); raw != "" {
		parsed, err := entityPkg.ParseFace(raw)
		if err != nil {
			writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", err.Error(), "/face")
			return entityPkg.CreateOptions{}, false
		}
		opts.Face = parsed
	}

	if msg := validateCreateIDOpts(def, opts.ID, opts.Prefix); msg != "" {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", msg, "")
		return entityPkg.CreateOptions{}, false
	}
	return opts, true
}

// gateCreateRelationAffordances applies the relation affordance gate to the
// edges riding a create body. Reports whether the request may proceed; writes
// the denial itself when not.
//
// Without it an edge the affordance layer refuses on the relation route could
// be written through the create body's `relations:` field. It runs before
// CreateEntity, so a denial never leaves the new entity written and unlinked.
// Verdicts resolve against the candidate's type and properties, so no id is needed.
func (h *writeHandler) gateCreateRelationAffordances(
	w http.ResponseWriter, r *http.Request,
	candidate *entityPkg.Entity, desired map[string]v1.RelationsUpdate,
) bool {
	if desired == nil {
		return true
	}
	err := h.affordances.validateRelationsModernAffordances(r.Context(), "", candidate, desired)
	if !h.relationGateOK(w, r, candidate, err) {
		return false
	}
	createReq := translateVerb("create", candidate.Type, "", candidate.Face)
	if !h.acl().AuthorizeWrite(r.Context(), createReq).Allow {
		return true
	}
	if _, _, err := h.checkRelationEdges(r.Context(), candidate.Type, desired); err != nil {
		h.writeRelationsValidationError(w, r, err)
		return false
	}
	return true
}

// relationGateOK answers the request itself for a failed
// [affordanceService.validateRelationsModernAffordances]: a 403 for a denial,
// with subject as its audit subject, the validation answer for a body the
// planner refused (face_required), and a 500 for a read fault. It reports
// whether the request may continue.
func (h *writeHandler) relationGateOK(
	w http.ResponseWriter, r *http.Request, subject *entityPkg.Entity, err error,
) bool {
	if err == nil {
		return true
	}
	var denial *AffordanceDenialError
	if errors.As(err, &denial) {
		h.denyAfford(r.Context(), w, subject, *denial)
		return false
	}
	var structural *structuralError
	var wire *v1.WireError
	var fault *gateFaultError
	if errors.As(err, &structural) || errors.As(err, &wire) || errors.As(err, &fault) {
		h.writeRelationsValidationError(w, r, err)
		return false
	}
	writeInternalError(w, r, "read_failed", "Failed to read relation source", err)
	return false
}

// relationSourcesOr500 is [affordanceService.relationSources] that answers the
// request itself when a source row cannot be read.
func (h *writeHandler) relationSourcesOr500(
	w http.ResponseWriter, r *http.Request, pathEntity *entityPkg.Entity, peer entityPkg.Ref, direction, relType string,
) ([]relationSource, bool) {
	sources, err := h.affordances.relationSources(r.Context(), pathEntity, peer, direction, relType)
	if err != nil {
		writeInternalError(w, r, "read_failed", "Failed to read relation source", err)
		return nil, false
	}
	return sources, true
}

// writeCreateRelations runs the two relation phases for a create: validate, then
// apply. Returns the accumulated soft warnings and whether the request may
// continue; writes the error response itself when not.
//
// Extracted from [writeHandler.handleV1CreateEntity] to keep it under the
// statement limit, and because the two phases are one unit: Phase A must run
// before any write so a structural relation error does not leave the entity
// half-linked (DEC-HWZHA atomicity).
//
// Note these run AFTER the entity exists, which is why the AFFORDANCE gate
// cannot live here — see [writeHandler.gateCreateRelationAffordances].
func (h *writeHandler) writeCreateRelations(
	w http.ResponseWriter, r *http.Request,
	created *entityPkg.Entity, desired map[string]v1.RelationsUpdate,
) (warnings []Warning, ok bool) {
	if desired == nil {
		return nil, true
	}
	ws, err := h.validateRelationsModern(r.Context(), created.ID, created.Type, desired)
	if err != nil {
		h.writeRelationsValidationError(w, r, err)
		return nil, false
	}
	warnings = ws

	ws, err = h.applyRelationsModern(r.Context(), created.Ref(), desired, nil)
	warnings = append(warnings, ws...)
	if err != nil {
		h.writeRelationsApplyError(w, r, err)
		return nil, false
	}
	return warnings, true
}

// handleV1CreateEntity creates an entity. The body is decoded strictly, so an
// unknown or misspelled key such as `face` fails instead of being ignored. The
// face reaches the manager only through CreateOptions, so the face it authorizes
// is the face it writes, and field write gates apply on create as on PATCH.
//
//archplan:auth=authz
func (h *writeHandler) handleV1CreateEntity(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	r = h.withProvision(r)

	var req struct {
		ID         string            `json:"id,omitempty"`
		Prefix     string            `json:"prefix,omitempty"`
		Face       string            `json:"face,omitempty"`
		World      string            `json:"world,omitempty"`
		Properties map[string]any    `json:"properties"`
		Content    string            `json:"content,omitempty"`
		Relations  v1.RelationsField `json:"relations"`
	}

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		var werr *v1.WireError
		if errors.As(err, &werr) {
			writeV1Error(w, r, http.StatusBadRequest, werr.Code, werr.Detail, werr.Path)
			return
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	s := h.schema()
	entityDef, defOK := s.Meta.Entities[typeName]
	if !defOK {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity type not found", typeName)
		return
	}

	rawFace, faceOK := h.createFace(w, r, req.Face, req.World, typeName, &entityDef)
	if !faceOK {
		return
	}

	createOpts, optsOK := parseCreateOpts(w, r, &entityDef, req.ID, req.Prefix, rawFace)
	if !optsOK {
		return
	}

	candidate := &entityPkg.Entity{Type: typeName, Face: createOpts.Face, Properties: req.Properties}
	if denial := h.affordances.validateCreateFieldWrite(r.Context(), candidate, req.Properties); denial != nil {
		h.denyAfford(r.Context(), w, candidate, *denial)
		return
	}

	if !h.gateCreateRelationAffordances(w, r, candidate, req.Relations.Modern) {
		return
	}

	createResult, err := h.manager.CreateEntity(r.Context(),
		&entityPkg.Entity{
			Type:       typeName,
			Properties: req.Properties,
			Content:    req.Content,
		},
		createOpts,
	)
	if err != nil {
		switch entitymanager.ClassifyCreateError(err) {
		case entitymanager.CreateErrForbidden:
			if writeForbiddenIfACLDenied(w, r, err) {
				return
			}
		case entitymanager.CreateErrFace:
			writeV1Error(w, r, http.StatusUnprocessableEntity, "face_required", err.Error(), "/face")
			return
		case entitymanager.CreateErrUnknown, entitymanager.CreateErrConflict, entitymanager.CreateErrInvalid:
		}
		writeWriteRefused(w, r, err)
		return
	}
	created := createResult.Entity

	relWarnings, ok := h.writeCreateRelations(w, r, created, req.Relations.Modern)
	if !ok {
		return
	}

	rels, visibleNeighbors, rerr := h.faceEdges.read(r.Context(), created)
	if rerr != nil {
		writeGateError(w, r, rerr)
		return
	}
	result := h.serializer.forWireScoped(r.Context(), created, rels, visibleNeighbors, s.Meta, plural)
	h.rollups.add(r.Context(), created.Type, []rollupRow{{ref: created.Ref(), props: result.Properties}},
		worldScopeFrom(r.Context()), nil)
	if len(relWarnings) > 0 {
		result.Warnings = append(result.Warnings, relWarnings...)
	}
	if len(createResult.Warnings) > 0 {
		result.Warnings = append(result.Warnings, createResult.Warnings...)
	}

	w.Header().Set("Location", fmt.Sprintf("/api/v1/%s/%s", plural, created.ID))

	writeV1JSON(w, http.StatusCreated, result)
}

// handleV1DryRunCreate evaluates field/option/relation affordances and
// soft validation against a candidate entity without persisting it, so the
// SPA create form can show them before commit (TKT-3I5U).
//
// It is read-shaped: no provision seam, one snapshot, no `denied-write` audit
// row and no write. The verdicts are advisory; the real create re-runs the
// affordance gate and is the sole authorization point. Relation edges are not
// staged, so relation affordances reflect the per-type verdict only.
//
//archplan:auth=authz
func (h *writeHandler) handleV1DryRunCreate(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	s := h.schema()

	var req struct {
		ID         string         `json:"id,omitempty"`
		Prefix     string         `json:"prefix,omitempty"`
		Face       string         `json:"face,omitempty"`
		World      string         `json:"world,omitempty"`
		Properties map[string]any `json:"properties"`
		Content    string         `json:"content,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	entityDef, ok := s.Meta.Entities[typeName]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity type not found", typeName)
		return
	}

	reqID := strings.TrimSpace(req.ID)
	reqPrefix := strings.TrimSpace(req.Prefix)

	rawDryFace := req.Face
	if world := strings.TrimSpace(req.World); world != "" && req.Face == "" &&
		len(entityDef.Faces) > 0 {

		if wdef, declared := s.Meta.Worlds[world]; declared {
			rawDryFace = wdef.Create
		}
	}

	var dryFace entityPkg.Face
	var faceWarning *Warning
	if raw := strings.TrimSpace(rawDryFace); raw != "" {
		parsed, perr := entityPkg.ParseFace(raw)
		if perr != nil {
			faceWarning = &Warning{Code: "face_invalid", Path: "/face", Detail: perr.Error()}
		} else {
			dryFace = parsed
		}
	}

	var idWarning *Warning
	if msg := validateCreateIDOpts(&entityDef, reqID, reqPrefix); msg != "" {
		idWarning = &Warning{Code: "id_opts_invalid", Path: "/id", Detail: msg}
	}

	candidate, warnings, err := h.manager.ValidateCreate(r.Context(),
		&entityPkg.Entity{Type: typeName, Properties: req.Properties, Content: req.Content},
		entityPkg.CreateOptions{ID: reqID, Prefix: reqPrefix, Face: dryFace},
	)
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}

	if def, ok := s.Meta.Entities[typeName]; ok {
		if candidate.Properties == nil {
			candidate.Properties = make(map[string]any)
		}
		for name := range def.Properties {
			if _, present := candidate.Properties[name]; !present {
				candidate.Properties[name] = nil
			}
		}
	}

	result := h.serializer.forWire(r.Context(), candidate, nil, h.schema().Meta, plural)
	maps.DeleteFunc(result.Actions, func(k string, _ bool) bool {
		return strings.HasPrefix(k, detailActionKeyPrefix)
	})
	createFields := computeFieldAffordancesFrom(createFieldVerdicts(
		r.Context(), h.serializer.affordances.resolver(), h.serializer.affordances.meta(), candidate))
	result.FieldAffordances = &createFields
	h.serializer.affordances.applyCreateLock(r.Context(), &result, candidate)
	if idWarning != nil {
		result.Warnings = append(result.Warnings, *idWarning)
	}
	if faceWarning != nil {
		result.Warnings = append(result.Warnings, *faceWarning)
	}
	if len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
	}

	writeV1JSON(w, http.StatusOK, result)
}

// writePatchError maps a failed PatchEntity to its HTTP status.
//
// A lost compare-and-swap race becomes a 412, the same status the If-Match
// compare gives, because the remedy is the same. Without If-Match the manager
// retries itself, so a surviving conflict is a 409. Matching is by errors.As,
// never on the message, because the manager wraps the store error
// (RR-HI9QIU).
func writePatchError(w http.ResponseWriter, r *http.Request, err error) {
	if writeForbiddenIfACLDenied(w, r, err) || writeFieldWriteRefused(w, err) {
		return
	}
	var conflict *store.VersionConflictError
	if errors.As(err, &conflict) {
		if r.Header.Get("If-Match") == "" {
			writeV1Error(w, r, http.StatusConflict, "conflict",
				"Entity is being modified concurrently", "retry the request")
			return
		}
		writeV1Error(w, r, http.StatusPreconditionFailed, "precondition_failed",
			"Entity has been modified", "concurrent write detected")
		return
	}
	writeWriteRefused(w, r, err)
}

// writeWriteRefused answers a refused entity create or update with a 422:
// conflict_block for content the store cannot hold ([writeConflictBlock]),
// validation_failed with the error text for anything else.
//
// A refusal by validation also lists each failure in `errors[]`, so a
// client can show it at the field it names (DEC-OYV2AK): the property's
// JSON pointer, or the empty pointer for a failure about the whole entity,
// such as a write-enforced rule.
func writeWriteRefused(w http.ResponseWriter, r *http.Request, err error) {
	if writeConflictBlock(w, r, err) {
		return
	}
	invalid, ok := errors.AsType[*entitymanager.ValidationError](err)
	if !ok {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}
	problem := v1.Error{
		Type:     "https://rela.dev/errors/validation_failed",
		Title:    "Validation failed",
		Status:   http.StatusUnprocessableEntity,
		Detail:   err.Error(),
		Instance: r.URL.Path,
		Errors:   validationFieldErrors(invalid),
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusUnprocessableEntity)
	_ = json.NewEncoder(w).Encode(problem)
}

// writeRuleRefusal answers a relation write that a write-enforced relation
// rule refused (TKT-EBARYBI) with the 422 an entity write gets, and reports
// whether it did. The problem names the rule and quotes its description,
// never an entity: it answers with the unwrapped [entitymanager.ValidationError],
// because a wrapping relation error names the request's first create target.
func writeRuleRefusal(w http.ResponseWriter, r *http.Request, err error) bool {
	invalid, ok := errors.AsType[*entitymanager.ValidationError](err)
	if !ok {
		return false
	}
	writeWriteRefused(w, r, invalid)
	return true
}

// validationFieldErrors maps each failure of a refused write to a
// [v1.FieldError]. The detail is the failure's own message, which never
// quotes a property value (see [metamodel.ValidationError]). A rule's
// failure also carries the properties it reads and its description.
func validationFieldErrors(invalid *entitymanager.ValidationError) []v1.FieldError {
	out := make([]v1.FieldError, 0, len(invalid.Errors))
	for _, e := range invalid.Errors {
		pointer := ""
		if e.Property != "" {
			pointer = "/properties/" + jsonPointerEscape(e.Property)
		}
		out = append(out, v1.FieldError{
			Source:  v1.ErrorSource{Pointer: pointer},
			Code:    string(e.Type),
			Detail:  e.Message,
			Fields:  e.Fields,
			Message: e.Description,
		})
	}
	return out
}

// jsonPointerEscape escapes one RFC 6901 reference token.
func jsonPointerEscape(s string) string {
	return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1")
}

// writeEarlyUpdateError answers an error from the PATCH's early
// [entityMutator.AuthorizeUpdate]. A row the manager's read gate refuses is
// the uniform 404 the resolver read gives, so the early check cannot tell a
// hidden row from a missing one. A denial is the write's 403. Anything else
// is a failed gate query.
func writeEarlyUpdateError(w http.ResponseWriter, r *http.Request, err error) {
	if writeForbiddenIfACLDenied(w, r, err) {
		return
	}
	if errors.Is(err, entitymanager.ErrEntityNotFound) {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return
	}
	writeGateError(w, r, err)
}

// currentVersions returns the field tokens of e as this caller sees it: the
// same face-scoped edges and redaction the GET applies.
func (h *writeHandler) currentVersions(
	ctx context.Context, e *entityPkg.Entity, plural string,
) (*v1.FieldVersions, error) {
	rels, visible, err := h.faceEdges.read(ctx, e)
	if err != nil {
		return nil, err
	}
	meta := h.schema().Meta
	view := h.serializer.forWireScoped(ctx, e, rels, visible, meta, plural)
	return fieldVersionsOf(&view, meta), nil
}

// visibleStored re-reads the row at ref through the GET's row and face
// gates. It reports false when the row is gone, has another type, or is
// hidden from this caller, and on a gate error: every caller then answers
// without the row's state, which a GET would not serve either.
func (h *writeHandler) visibleStored(
	ctx context.Context, typeName string, ref entityPkg.Ref,
) (*entityPkg.Entity, bool) {
	e, found, err := h.readVisible(ctx, typeName, ref)
	if err != nil || !found || e.Type != typeName {
		return nil, false
	}
	return e, true
}

// preconditionScope is the set of fields a PATCH writes.
type preconditionScope struct {
	props     map[string]any
	unset     []string
	content   bool
	relations bool
}

// checkPreconditions validates a PATCH's preconditions against the fields it
// writes and compares them with the tokens of entity, the row this handler
// read. It writes the 400, the 412 or the edge-read failure, and returns
// false when the write must not proceed. A nil pre always passes.
func (h *writeHandler) checkPreconditions(
	w http.ResponseWriter, r *http.Request, pre *v1.Preconditions, scope preconditionScope,
	entity *entityPkg.Entity, plural string,
) bool {
	if pre == nil {
		return true
	}
	if ptr, ok := validatePreconditionScope(pre, scope); !ok {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_precondition",
			"Precondition names a field this request does not write", ptr)
		return false
	}
	versions, err := h.currentVersions(r.Context(), entity, plural)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	if conflicts := preconditionConflicts(pre, versions, entity.ID, entity.Type); conflicts != nil {
		writeFieldConflict(w, r, conflicts, versions)
		return false
	}
	return true
}

// writeLostRace answers a PATCH with preconditions whose store
// compare-and-swap lost to a concurrent write. It re-reads the row and
// reports which preconditions now fail, which may be none. It does not retry.
// Returns false when err is not a lost race. When the row is gone or now
// hidden it answers a bare 412 without tokens, because a hidden row's tokens
// would disclose its new values.
func (h *writeHandler) writeLostRace(
	w http.ResponseWriter, r *http.Request, err error, pre *v1.Preconditions,
	typeName string, ref entityPkg.Ref, plural string,
) bool {
	var conflict *store.VersionConflictError
	if !errors.As(err, &conflict) {
		return false
	}
	current, found := h.visibleStored(r.Context(), typeName, ref)
	var versions *v1.FieldVersions
	if found {
		versions, err = h.currentVersions(r.Context(), current, plural)
	}
	if !found || err != nil {
		writeV1Error(w, r, http.StatusPreconditionFailed, "precondition_failed",
			"Entity has been modified", "concurrent write detected")
		return true
	}
	conflicts := preconditionConflicts(pre, versions, current.ID, current.Type)
	if conflicts == nil {
		conflicts = &v1.FieldConflicts{}
	}
	writeFieldConflict(w, r, conflicts, versions)
	return true
}

// writeFieldConflict writes the 412 for failed per-field preconditions, with
// the failed fields and the current tokens as problem+json extension members.
func writeFieldConflict(
	w http.ResponseWriter, r *http.Request, conflicts *v1.FieldConflicts, versions *v1.FieldVersions,
) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusPreconditionFailed)
	_ = json.NewEncoder(w).Encode(v1.Error{
		Type:      "https://rela.dev/errors/precondition_failed",
		Title:     "Entity has been modified",
		Status:    http.StatusPreconditionFailed,
		Detail:    "field changed since it was read",
		Instance:  r.URL.Path,
		Conflicts: conflicts,
		Versions:  versions,
	})
}

// handleV1UpdateEntity patches one entity. The resolver read runs before body
// parsing, authorization, If-Match and the lock check, so a hidden entity or a
// denied face gets the same 404 as a missing one; an earlier 400, 403, 412 or
// 422 would be an existence oracle. The write is then authorized once.
//
//nolint:gocognit,funlen // update handler threads the validation-policy classes (400/422/200-with-warnings) through each field; the branches are the documented write-policy cases, not extractable shared logic.
//archplan:auth=authz
func (h *writeHandler) handleV1UpdateEntity(w http.ResponseWriter, r *http.Request, typeName, plural, entityID string) {
	r = h.withProvision(r)

	s := h.schema()

	entity, found := writeTargetOr404(w, r, h.visible, typeName, entityID)
	if !found {
		return
	}
	ref := entity.Ref()

	if entity.IsLocked() {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "encrypted_inaccessible",
			"Cannot edit an inaccessible entity", "File is git-crypt encrypted; run `git-crypt unlock` first.")
		return
	}

	var req struct {
		Properties      map[string]any    `json:"properties,omitempty"`
		PropertiesUnset []string          `json:"properties_unset,omitempty"`
		Content         *string           `json:"content,omitempty"`
		Relations       v1.RelationsField `json:"relations"`
		Preconditions   *v1.Preconditions `json:"preconditions,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		var werr *v1.WireError
		if errors.As(err, &werr) {
			writeV1Error(w, r, http.StatusBadRequest, werr.Code,
				werr.Detail, werr.Path)
			return
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	entityChanged := req.Properties != nil || len(req.PropertiesUnset) > 0 || req.Content != nil
	var grant entitymanager.UpdateGrant
	if entityChanged {
		g, err := h.manager.AuthorizeUpdate(r.Context(), entity)
		if err != nil {
			writeEarlyUpdateError(w, r, err)
			return
		}
		grant = g
	}

	if ifMatch := r.Header.Get("If-Match"); ifMatch != "" {
		if ifMatch != h.computeETag(r.Context(), entity) {
			writeV1Error(w, r, http.StatusPreconditionFailed, "precondition_failed",
				"Entity has been modified", "ETag mismatch")
			return
		}
	}

	if denial := h.affordances.validateFieldWrite(
		r.Context(), entity, req.Properties, req.PropertiesUnset,
	); denial != nil {
		h.denyAfford(r.Context(), w, entity, *denial)
		return
	}
	if req.Relations.Modern != nil {
		err := h.affordances.validateRelationsModernAffordances(r.Context(), ref.ID, entity, req.Relations.Modern)
		if !h.relationGateOK(w, r, entity, err) {
			return
		}
	}

	scope := preconditionScope{
		props: req.Properties, unset: req.PropertiesUnset,
		content: req.Content != nil, relations: req.Relations.Modern != nil,
	}
	if !h.checkPreconditions(w, r, req.Preconditions, scope, entity, plural) {
		return
	}

	var warnings []Warning
	if req.Relations.Modern != nil {
		ws, err := h.validateRelationsModern(r.Context(), ref.ID, entity.Type, req.Relations.Modern)
		if err != nil {
			h.writeRelationsValidationError(w, r, err)
			return
		}
		warnings = ws
	}

	if len(req.PropertiesUnset) > 0 {
		entityTypeDef, hasType := s.Meta.Entities[entity.Type]
		for i, k := range req.PropertiesUnset {
			if hasType {
				if _, declared := entityTypeDef.Properties[k]; !declared {
					warnings = append(warnings, Warning{
						Code:   "unknown_property_unset_key",
						Path:   fmt.Sprintf("/properties_unset/%d", i),
						Detail: fmt.Sprintf("property %q is not declared on entity type %q", k, entity.Type),
					})
				}
			}
		}
	}
	joined, planErr := planJoinedRelations(r.Context(), h, ref, entityChanged, req.Relations.Modern)
	if planErr != nil {
		h.writeRelationsApplyError(w, r, planErr)
		return
	}
	if entityChanged {
		patch := entityPkg.Patch{
			Properties:      req.Properties,
			MetaUnset:       req.PropertiesUnset,
			Content:         req.Content,
			ExpectedVersion: string(store.VersionOf(entity)),
		}
		stateRef := entityPkg.FormatStateRef(entity.ID, entity.Face)
		updateResult, err := patchJoined(r.Context(), h, grant, stateRef, patch, joined)
		if err != nil {
			writeUpdateError(w, r, h, err, joined, req.Preconditions, typeName, ref, plural)
			return
		}
		if updateResult != nil && updateResult.Entity != nil {
			entity = updateResult.Entity
		}
		if updateResult != nil {
			warnings = append(warnings, updateResult.Warnings...)
		}
	}

	if req.Relations.Modern != nil {
		ws, err := h.applyRelationsModern(r.Context(), ref, req.Relations.Modern, joined)
		warnings = append(warnings, ws...)
		if err != nil {
			h.writeRelationsApplyError(w, r, err)
			return
		}
	}

	rels, visibleNeighbors, rerr := h.faceEdges.read(r.Context(), entity)
	if rerr != nil {
		writeGateError(w, r, rerr)
		return
	}
	served, versioned := h.visibleStored(r.Context(), typeName, ref)
	if !versioned {
		served = entity
	}
	result := h.serializer.forWireScoped(r.Context(), served, rels, visibleNeighbors, s.Meta, plural)
	if len(warnings) > 0 {
		result.Warnings = warnings
	}
	if versioned {
		result.Versions = fieldVersionsOf(&result, s.Meta)
	}
	h.rollups.add(r.Context(), served.Type, []rollupRow{{ref: served.Ref(), props: result.Properties}},
		worldScopeFrom(r.Context()), nil)
	w.Header().Set("ETag", servedETag(r.Context(), h.affordances, served, rels))

	writeV1JSON(w, http.StatusOK, result)
}

// handleV1DeleteEntity deletes the addressed entity. The resolver read runs
// before AuthorizeWrite, so a hidden target or a denied face gets a 404, never
// a 403 that names a rule.
//
//archplan:auth=authz
func (h *writeHandler) handleV1DeleteEntity(w http.ResponseWriter, r *http.Request, typeName, _, entityID string) {
	r = h.withProvision(r)

	entity, found := writeTargetOr404(w, r, h.visible, typeName, entityID)
	if !found {
		return
	}
	ref := entity.Ref()

	var err error
	switch {
	case r.Header.Get("If-Match") != "":
		c := conditionalDelete{
			reader: h.reader, visibleStored: h.visibleStored, computeETag: h.computeETag,
			deletes: h.ifMatchDeletes, soft: h.softDeletes != nil,
		}
		version, ok := c.ifMatch(w, r, typeName, ref)
		if !ok {
			return
		}
		err = c.delete(r.Context(), ref, version)
	case ref.Face.IsImplicit():
		err = h.deleteWholeEntity(r.Context(), ref.ID)
	default:
		_, err = h.manager.DeleteEntityFace(r.Context(), ref.ID, ref.Face, true)
	}
	if err != nil {
		if writeForbiddenIfACLDenied(w, r, err) {
			return
		}
		var blocked *entitymanager.AnchorBlockedError
		if errors.As(err, &blocked) {
			writeV1Error(w, r, http.StatusConflict, "channel_blocks_delete",
				"A channel attached to this entity blocks its deletion",
				"channel type "+blocked.ChannelType+" has on_entity_delete: block")
			return
		}
		if errors.Is(err, entitymanager.ErrConcurrentChange) {
			writeV1Error(w, r, http.StatusConflict, "conflict", "Entity is being modified concurrently",
				"retry the delete")
			return
		}
		if _, ok := errors.AsType[*store.VersionConflictError](err); ok {
			writeV1Error(w, r, http.StatusPreconditionFailed, "precondition_failed",
				"Entity has been modified", "concurrent write detected")
			return
		}
		if errors.Is(err, entitymanager.ErrEntityNotFound) {
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return
		}
		writeInternalError(w, r, "delete_failed", "Failed to delete entity", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ownedSource resolves the source row and tail of an edge the path entity
// is the source of. An identity-scoped edge belongs to the entity, so its
// tail is the implicit face whatever the address, and served stays the
// source. A content-scoped edge belongs to one face, which the address must
// name on a faced type ([writeTargetOr404]); a bare id there is
// `face_required`, never the face a world ranks first. It writes the
// response and reports false when the request stops here.
func (h *writeHandler) ownedSource(
	w http.ResponseWriter, r *http.Request, typeName, addr, relType string, served *entityPkg.Entity,
) (*entityPkg.Entity, entityPkg.Face, bool) {
	if !metamodel.IsContentScoped(h.schema().Meta, relType) {
		return served, entityPkg.ImplicitFace, true
	}
	e, ok := writeTargetOr404(w, r, h.visible, typeName, addr)
	if !ok {
		return nil, "", false
	}
	return e, e.Face, true
}

// bodyPeer resolves the entity a relation create's body names. On an
// outgoing edge it is the target, and a target is always the whole entity,
// so the body is a bare id. On an incoming edge it is the source: an
// identity-scoped edge belongs to the entity, so its tail is the implicit
// face, and a content-scoped one belongs to the face the body names
// (`POL-1@draft`), resolved as every content write is
// ([visibility.Resolver.WriteTarget]). A bare id of a faced source is
// `face_required`. It writes the response and reports false when the request
// stops here.
func (h *writeHandler) bodyPeer(
	w http.ResponseWriter, r *http.Request, relType, addr, direction string,
) (entityPkg.Ref, bool) {
	ctx := r.Context()
	if direction != string(DirectionIncoming) || !metamodel.IsContentScoped(h.schema().Meta, relType) {
		parsed, err := entityPkg.ParseAddress(addr)
		if err != nil || (direction != string(DirectionIncoming) && parsed.ID() != addr) {
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return entityPkg.Ref{}, false
		}
		if !familyReadableOr404(w, r, h.visible, parsed.ID()) {
			return entityPkg.Ref{}, false
		}
		return entityPkg.Ref{ID: parsed.ID(), Face: entityPkg.ImplicitFace}, true
	}
	parsed, err := entityPkg.ParseAddress(addr)
	if err != nil {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return entityPkg.Ref{}, false
	}
	typ, err := h.visible.readableType(ctx, parsed.ID())
	if err != nil {
		writeGateError(w, r, err)
		return entityPkg.Ref{}, false
	}
	if typ == "" {
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return entityPkg.Ref{}, false
	}
	ref, ok, err := h.visible.resolver.WriteTarget(ctx, worldFromContext(ctx).visibility(), typ, parsed)
	var amb *visibility.AmbiguousAddressError
	switch {
	case errors.As(err, &amb):
		writeFaceRequired(w, r, amb)
		return entityPkg.Ref{}, false
	case err != nil:
		writeGateError(w, r, err)
		return entityPkg.Ref{}, false
	case !ok:
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
		return entityPkg.Ref{}, false
	}
	return ref, true
}

// edgeSource is the source row and tail the single-relation PATCH and DELETE
// routes address: [writeHandler.ownedSource] when the path entity is the
// edge's source, else served with the stored edge's own tail
// ([writeHandler.incomingEdgeTail]).
func (h *writeHandler) edgeSource(
	w http.ResponseWriter, r *http.Request, typeName, addr string, served *entityPkg.Entity,
	peer entityPkg.Address, from, relType, to string,
) (*entityPkg.Entity, entityPkg.Face, bool) {
	if from == served.ID {
		return h.ownedSource(w, r, typeName, addr, relType, served)
	}
	tail, err := h.incomingEdgeTail(r.Context(), peer, relType, to)
	if err != nil {
		var fault *gateFaultError
		var structural *structuralError
		switch {
		case errors.Is(err, errEdgeNotFound):
			writeV1Error(w, r, http.StatusNotFound, "relation_not_found", "Relation not found", "")
			return nil, "", false
		case errors.As(err, &fault) || errors.As(err, &structural):
			h.writeRelationsValidationError(w, r, err)
			return nil, "", false
		}
		writeInternalError(w, r, "read_failed", "Failed to read relation", err)
		return nil, "", false
	}
	return served, tail, true
}

// relationTargetID is the entity id of a single-relation route's target
// segment, gated before the direction is known. Only an incoming target may
// name a face: it is the edge's source, and a content-scoped edge belongs to
// one face of it ([peerAddress]).
func relationTargetID(targetID string) string {
	if addr, err := entityPkg.ParseAddress(targetID); err == nil {
		return addr.ID()
	}
	return targetID
}

// writeRelationsValidationError maps a Phase A validation error from
// the modern reconciler to the corresponding HTTP response. v1.WireError
// → 400 (caller bug); structuralError → 422 (storage can't represent).
func (h *writeHandler) writeRelationsValidationError(w http.ResponseWriter, r *http.Request, err error) {
	var werr *v1.WireError
	if errors.As(err, &werr) {
		writeV1Error(w, r, http.StatusBadRequest, werr.Code, werr.Detail, werr.Path)
		return
	}
	if se, ok := asStructuralError(err); ok {
		writeV1Error(w, r, http.StatusUnprocessableEntity, se.Code, se.Detail, se.Path)
		return
	}
	var gerr *gateFaultError
	if errors.As(err, &gerr) {
		writeGateError(w, r, gerr.err)
		return
	}
	writeV1Error(w, r, http.StatusUnprocessableEntity,
		"relation_failed", "Failed to validate relations", err.Error())
}

// writeRelationsApplyError maps a Phase C write error to a 500 — the
// entity may already have been updated, so a partial state is on disk.
// This is the documented atomicity gap. ACL denials short-circuit to
// the structured 403 path; a dangling-peer structuralError maps to 422
// (the reference did not resolve, so the edge was not stored —
// BUG-K6FEVB); an edge that concurrent requests kept creating and deleting
// under this one maps to 409; everything else is a 500 whose detail names
// the relation, op and target, with the cause in the server log.
func (h *writeHandler) writeRelationsApplyError(w http.ResponseWriter, r *http.Request, err error) {
	if writeForbiddenIfACLDenied(w, r, err) || writeRuleRefusal(w, r, err) {
		return
	}
	detail := reconcileDetail(err)
	if errors.Is(err, entitymanager.ErrRelationAlreadyExists) || errors.Is(err, entitymanager.ErrRelationNotFound) {
		writeV1Error(w, r, http.StatusConflict, "conflict",
			"A concurrent request changed the same relation; retry", detail)
		return
	}
	if se, ok := asStructuralError(err); ok {
		writeV1Error(w, r, http.StatusUnprocessableEntity, se.Code, se.Detail, se.Path)
		return
	}
	if detail != "" {
		detail += "; "
	}
	if errors.Is(err, store.ErrConflictBlock) {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "conflict_block",
			conflictBlockTitle+"; the entity may have been saved", detail+store.ErrConflictBlock.Error())
		return
	}
	writeInternalErrorDetail(w, r, "relation_write_failed",
		"Failed to apply relation changes after entity update; the entity may have been updated",
		detail+internalErrorDetail, err)
}

// handleV1CreateRelation adds an edge to the path entity. The path entity is
// read-gated before the body is parsed and before the affordance check, and the
// body's target is read-gated too, so a hidden entity at either end gets the
// same 404 as a missing one.
//
//archplan:auth=authz
func (h *writeHandler) handleV1CreateRelation(
	w http.ResponseWriter, r *http.Request, typeName, addr, relType string,
) {
	r = h.withProvision(r)

	entity, found := readAddressedOr404(w, r, h.visible, typeName, addr)
	if !found {
		return
	}

	var req struct {
		ID        string         `json:"id"`
		Meta      map[string]any `json:"meta,omitempty"`
		Direction string         `json:"direction,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	if req.ID == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_id", "Target ID is required", "")
		return
	}

	peer, ok := h.bodyPeer(w, r, relType, req.ID, req.Direction)
	if !ok {
		return
	}

	newTail := peer.Face
	if req.Direction != string(DirectionIncoming) {
		if entity, newTail, ok = h.ownedSource(w, r, typeName, addr, relType, entity); !ok {
			return
		}
	}

	sources, ok := h.relationSourcesOr500(w, r, entity, peer, req.Direction, relType)
	if !ok {
		return
	}
	source, denial := h.affordances.relationOpDenial(r.Context(), sources, relType, RelationOpCreate)
	if denial != nil {
		h.denyAfford(r.Context(), w, source, *denial)
		return
	}
	source, denial = h.affordances.relationMetaDenial(r.Context(), sources, relType, req.Meta, nil)
	if denial != nil {
		h.denyAfford(r.Context(), w, source, *denial)
		return
	}

	from, to := resolveRelationEndpoints(entity.ID, peer.ID, req.Direction)

	_, err := h.manager.CreateRelation(r.Context(),
		entityPkg.RelationKey{From: from, FromFace: newTail, Type: relType, To: to},
		entityPkg.RelationOptions{Properties: req.Meta},
	)
	if err != nil {
		if writeForbiddenIfACLDenied(w, r, err) || writeCardinalityIfExceeded(w, r, err) ||
			writeRuleRefusal(w, r, err) {

			return
		}
		if te := endpointTypeRefusal(err, relType); te != nil {
			writeV1Error(w, r, http.StatusUnprocessableEntity, te.Code, "Relation endpoint type not allowed", te.Detail)
			return
		}
		writeV1Error(w, r, http.StatusUnprocessableEntity, "relation_failed", "Failed to create relation", err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// writeCardinalityIfExceeded answers a create the relation's max_outgoing or
// max_incoming refused (TKT-65LVAK) with 422 cardinality_exceeded, and
// reports whether it did.
func writeCardinalityIfExceeded(w http.ResponseWriter, r *http.Request, err error) bool {
	var ce *entitymanager.CardinalityError
	if !errors.As(err, &ce) {
		return false
	}
	writeV1Error(w, r, http.StatusUnprocessableEntity, "cardinality_exceeded",
		"Relation cardinality exceeded", ce.Error())
	return true
}

// handleV1UpdateRelation changes an edge's meta. The path entity and the peer
// are both read-gated before the edge or its affordances are touched, so a
// hidden peer gets the same 404 as an absent one.
//
//archplan:auth=authz
func (h *writeHandler) handleV1UpdateRelation(
	w http.ResponseWriter, r *http.Request, typeName, addr, relType, targetID string,
) {
	r = h.withProvision(r)

	entity, found := readAddressedOr404(w, r, h.visible, typeName, addr)
	if !found {
		return
	}
	if !familyReadableOr404(w, r, h.visible, relationTargetID(targetID)) {
		return
	}

	var req struct {
		Meta      map[string]any        `json:"meta"`
		Direction string                `json:"direction,omitempty"`
		Position  *relationPositionWire `json:"position,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	peer := peerAddress(targetID, req.Direction == string(DirectionIncoming))
	from, to := resolveRelationEndpoints(entity.ID, peer.ID(), req.Direction)

	entity, tail, ok := h.edgeSource(w, r, typeName, addr, entity, peer, from, relType, to)
	if !ok {
		return
	}

	sources, ok := h.relationSourcesOr500(w, r, entity, entityPkg.Ref{ID: from, Face: tail}, req.Direction, relType)
	if !ok {
		return
	}
	key := entityPkg.RelationKey{From: from, FromFace: tail, Type: relType, To: to}
	expected, ok := relationIfMatch(w, r, h.store, h.affordances, entity, key)
	if !ok {
		return
	}
	if req.Position != nil {
		writeRelationPosition(w, r, h, relationPositionRequest{
			key:       key,
			position:  *req.Position,
			incoming:  req.Direction == string(DirectionIncoming),
			sources:   sources,
			metaGiven: len(req.Meta) > 0,
		})
		return
	}
	if source, denial := h.affordances.relationMetaDenial(r.Context(), sources, relType, req.Meta, nil); denial != nil {
		h.denyAfford(r.Context(), w, source, *denial)
		return
	}

	if !orderMetaValidOr400(w, r, h.schema().Meta, relType, req.Meta) {
		return
	}

	rel, err := h.manager.UpdateRelation(r.Context(), key,
		entityPkg.RelationOptions{Properties: req.Meta, ExpectedVersion: expected})
	if err != nil {
		if writeForbiddenIfACLDenied(w, r, err) {
			return
		}
		var conflict *store.VersionConflictError
		if errors.As(err, &conflict) {
			writeV1Error(w, r, http.StatusPreconditionFailed, "precondition_failed",
				"Relation has been modified", "concurrent write detected")
			return
		}
		writeV1Error(w, r, http.StatusNotFound, "relation_not_found", "Relation not found", err.Error())
		return
	}

	view, etag := relationView(r.Context(), h.affordances, entity, rel)
	w.Header().Set("ETag", etag)
	result := map[string]any{
		"from": rel.From,
		"type": rel.Type,
		"to":   rel.To,
	}
	if len(view.Properties) > 0 {
		result["meta"] = view.Properties
	}

	writeV1JSON(w, http.StatusOK, result)
}

// orderMetaValidOr400 checks that the managed order properties in meta are
// finite numbers when present, and answers 400 when one is not. It runs
// before the write so a wire-format error does not surface as a 422 from
// the manager. It is a package function for the same reason as
// relationIfMatch.
func orderMetaValidOr400(
	w http.ResponseWriter, r *http.Request, mm *metamodel.Metamodel, relType string, meta map[string]any,
) bool {
	relDef, ok := mm.Relations[relType]
	if !ok {
		return true
	}
	for _, prop := range []string{metamodel.OrderPropertyOut, metamodel.OrderPropertyIn} {
		if (prop == metamodel.OrderPropertyOut && relDef.OutgoingOrderProperty() == "") ||
			(prop == metamodel.OrderPropertyIn && relDef.IncomingOrderProperty() == "") {

			continue
		}
		v, present := meta[prop]
		if !present {
			continue
		}
		if _, ok := entitymanager.FiniteOrder(v); !ok {
			writeV1Error(w, r, http.StatusBadRequest, "order_value_invalid",
				"managed order property must be a finite number", prop)
			return false
		}
	}
	return true
}

// relationGetter reads one stored relation.
type relationGetter interface {
	GetRelation(ctx context.Context, k entityPkg.RelationKey) (*entityPkg.Relation, error)
}

// relationIfMatch compares a relation PATCH's or DELETE's If-Match with the ETag a GET of
// the edge would serve this caller, and answers 412 on a mismatch. On a match
// it returns the stored relation's version, which the manager compares again
// inside its transaction, so a write landing between this read and the update
// is a 412 too. The compare is against the caller's view, never the raw row,
// so a caller cannot test guesses of hidden meta.
//
// Without If-Match it returns "" and the update is unconditional. It is a
// package function so writeHandler stays under its plimsoll method cap.
func relationIfMatch(
	w http.ResponseWriter, r *http.Request, rels relationGetter, aff affordanceService,
	src *entityPkg.Entity, key entityPkg.RelationKey,
) (string, bool) {
	ifMatch := r.Header.Get("If-Match")
	if ifMatch == "" {
		return "", true
	}
	raw, err := rels.GetRelation(r.Context(), key)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeV1Error(w, r, http.StatusNotFound, "relation_not_found", "Relation not found", "")
			return "", false
		}
		writeGateError(w, r, err)
		return "", false
	}
	if _, etag := relationView(r.Context(), aff, src, raw); etag != ifMatch {
		writeV1Error(w, r, http.StatusPreconditionFailed, "precondition_failed",
			"Relation has been modified", "ETag mismatch")
		return "", false
	}
	return string(store.RelationVersionOf(raw)), true
}

// handleV1DeleteRelation removes an edge. The path entity and the peer are both
// read-gated before the edge or its affordances are touched, so a hidden peer
// gets the same 404 as an absent one.
//
//archplan:auth=authz
func (h *writeHandler) handleV1DeleteRelation(
	w http.ResponseWriter, r *http.Request, typeName, addr, relType, targetID string,
) {
	r = h.withProvision(r)

	entity, found := readAddressedOr404(w, r, h.visible, typeName, addr)
	if !found {
		return
	}
	if !familyReadableOr404(w, r, h.visible, relationTargetID(targetID)) {
		return
	}

	direction := r.URL.Query().Get("direction")
	peer := peerAddress(targetID, direction == string(DirectionIncoming))
	from, to := resolveRelationEndpoints(entity.ID, peer.ID(), direction)

	entity, tail, ok := h.edgeSource(w, r, typeName, addr, entity, peer, from, relType, to)
	if !ok {
		return
	}

	sources, ok := h.relationSourcesOr500(w, r, entity, entityPkg.Ref{ID: from, Face: tail}, direction, relType)
	if !ok {
		return
	}
	source, denial := h.affordances.relationOpDenial(r.Context(), sources, relType, RelationOpRemove)
	if denial != nil {
		h.denyAfford(r.Context(), w, source, *denial)
		return
	}

	key := entityPkg.RelationKey{From: from, FromFace: tail, Type: relType, To: to}
	version, ok := relationIfMatch(w, r, h.store, h.affordances, entity, key)
	if !ok {
		return
	}

	var err error
	if version != "" {
		err = h.ifMatchDeletes.DeleteRelation(r.Context(), key, store.EntityVersion(version))
	} else {
		err = h.manager.DeleteRelation(r.Context(), key)
	}
	if err != nil {
		if writeForbiddenIfACLDenied(w, r, err) || writeRuleRefusal(w, r, err) {
			return
		}
		if _, ok := errors.AsType[*store.VersionConflictError](err); ok {
			writeV1Error(w, r, http.StatusPreconditionFailed, "precondition_failed",
				"Relation has been modified", "concurrent write detected")
			return
		}
		writeV1Error(w, r, http.StatusNotFound, "relation_not_found", "Relation not found", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleV1CloneEntity copies an entity. A hidden source, or a face the caller
// may not read, gets the same 404 as a missing source.
//
//archplan:auth=authz
func (h *writeHandler) handleV1CloneEntity(
	w http.ResponseWriter, r *http.Request, typeName, addr string,
) {
	r = h.withProvision(r)

	s := h.schema()

	entity, found := readAddressedOr404(w, r, h.visible, typeName, addr)
	if !found {
		return
	}

	props := make(map[string]any)
	maps.Copy(props, entity.Properties)
	for _, prop := range metamodel.FileProperties(s.Meta, typeName) {
		delete(props, prop)
	}

	cloneResult, err := h.manager.CreateEntity(r.Context(),
		&entityPkg.Entity{
			Type:       typeName,
			Properties: props,
			Content:    entity.Content,
		},
		entityPkg.CreateOptions{Face: entity.Face},
	)
	if err != nil {
		if writeForbiddenIfACLDenied(w, r, err) {
			return
		}
		var verr *entitymanager.ValidationError
		if errors.As(err, &verr) {
			writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", verr.Error())
			return
		}
		writeInternalError(w, r, "clone_failed", "Failed to clone entity", err)
		return
	}
	newEntity := cloneResult.Entity

	entityDef := s.Meta.Entities[typeName]
	plural := entityDef.GetPlural(typeName)
	result := h.serializer.forWire(r.Context(), newEntity, nil, h.schema().Meta, plural)

	w.Header().Set("Location", fmt.Sprintf("/api/v1/%s/%s", plural, newEntity.ID))
	writeV1JSON(w, http.StatusCreated, result)
}

// handleV1ConflictResolve applies a conflict resolution. It needs git:sync,
// then the write check of [writeHandler.authorizeConflictResolve], then the
// field check of [conflictFieldDenial].
// The read gate runs before anything else reads the file, so a file that names
// a hidden row gets the same 404 as a missing one. Errors name the rule, never
// a field value.
//
//archplan:auth=authz reason="requires git:sync, then re-authorizes the write"
func (h *writeHandler) handleV1ConflictResolve(w http.ResponseWriter, r *http.Request) {
	if !permitsGitSync(h.acl(), r) {
		writeGitSyncForbidden(w, r, "Resolving a merge conflict")
		return
	}
	var req v1.ConflictResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid JSON", err.Error())
		return
	}

	if req.Path == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_path", "Path is required", "")
		return
	}

	absPath, ok := resolveConflictPath(h.paths, w, r, req.Path)
	if !ok {
		return
	}

	r = h.withProvision(r)

	st := h.schema()

	cf, ok := parseConflictOr404(w, r, absPath, st.Meta)
	if !ok {
		return
	}
	id := identifyConflict(h.paths, st.Meta, cf)
	if !conflictReadableOr404(w, r, h.visible, id) {
		return
	}

	resolution := &conflict.Resolution{
		PropertyChoices: make(map[string]conflict.Side),
	}

	for prop, choice := range req.PropertyChoices {
		if choice == "theirs" {
			resolution.PropertyChoices[prop] = conflict.SideTheirs
		} else {
			resolution.PropertyChoices[prop] = conflict.SideOurs
		}
	}

	switch req.ContentChoice {
	case "theirs":
		resolution.ContentChoice = conflict.SideTheirs
	case "manual":
		resolution.ManualContent = req.ManualContent
	default:
		resolution.ContentChoice = conflict.SideOurs
	}

	resolvedEntity, resolvedRelation, err := conflict.Resolve(cf, resolution)
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "conflict_unresolvable",
			"The conflict cannot be resolved automatically", "Resolve it by editing the file")
		return
	}
	if !resolvedMatchesFile(id, resolvedEntity, resolvedRelation) {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "conflict_identity_mismatch",
			"The resolved side names another entity or edge than the file", "Resolve it by editing the file")
		return
	}
	if !h.authorizeConflictResolve(w, r, id, resolvedRelation) {
		return
	}
	subject, denial, err := conflictFieldDenial(r.Context(), h.affordances, id, cf, resolvedEntity, resolvedRelation)
	switch {
	case errors.Is(err, errConflictSourceUnreadable):
		writeConflictNotFound(w, r)
		return
	case err != nil:
		writeGateError(w, r, err)
		return
	case denial != nil:
		h.denyAfford(r.Context(), w, subject, *denial)
		return
	}
	if err := conflict.ValidateResolved(resolvedEntity, st.Meta); err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}
	if err := conflict.WriteResolved(absPath, resolvedEntity, resolvedRelation); err != nil {
		writeInternalError(w, r, "resolve_failed", "Failed to resolve", err)
		return
	}
	h.recordConflictResolveAudit(r.Context(), req.Path, resolvedEntity, resolvedRelation)

	writeV1JSON(w, http.StatusOK, map[string]any{
		"success": true,
		"path":    req.Path,
	})
}

// resolvedMatchesFile reports whether the resolution names the row or edge
// its file's path names. The write check judges the resolved identity and
// the write replaces the file, so the two must be the same, or the check
// would judge a different row from the one written. A relation gets the
// file's tail face here, which its frontmatter does not carry.
func resolvedMatchesFile(id conflictIdentity, e *entityPkg.Entity, rel *entityPkg.Relation) bool {
	if rel != nil {
		if len(id.rels) == 0 || rel.From != id.edge.From || rel.Type != id.edge.Type || rel.To != id.edge.To {
			return false
		}
		rel.FromFace = id.edge.FromFace
		return true
	}
	return len(id.rows) > 0 && e.ID == id.file.ID && e.Type == id.file.Type
}

// authorizeConflictResolve authorizes the write a conflict resolution
// performs, after the caller's read gate has passed and
// [resolvedMatchesFile] holds. An entity file is an update of the row at the
// file's id and face, asked of [entityMutator.AuthorizeUpdate], so the
// manager runs its read gate, write check and denied-write audit (BUG-0E6S7Q).
// A relation file gates like a relation update on the source-entity type. A
// deny records a `denied-write` audit row and writes the standard 403 body.
// Returns true when the write may proceed.
//
//archplan:guard=authz reason="conflict resolution writes the file itself, so it authorizes the write here"
func (h *writeHandler) authorizeConflictResolve(
	w http.ResponseWriter, r *http.Request, id conflictIdentity, rel *entityPkg.Relation,
) bool {
	if rel == nil {
		return authorizeConflictEntity(w, r, h.manager, id)
	}
	ctx := r.Context()
	fromType := storedTypeOf(ctx, h.store, rel.From)
	aclReq := translateRelationWrite(h.schema().Meta, rel.Type, fromType, rel.From, rel.FromFace)
	decision := h.acl().AuthorizeWrite(ctx, aclReq)
	if decision.Allow {
		return true
	}
	h.audit().Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpDeniedWrite,
		Subject:     conflictAuditSubject(nil, rel),
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		AssertedBy:  audit.AssertedByFrom(ctx),
		Summary: fmt.Sprintf("denied: %s (rule_kind=%s rule_id=%s op=conflict-resolve)",
			decision.Reason, decision.RuleKind, decision.RuleID),
	})
	writeForbiddenIfACLDenied(w, r, &acl.ForbiddenError{Decision: decision})
	return false
}

// authorizeConflictEntity is the entity half of
// [writeHandler.authorizeConflictResolve], asked of m.
func authorizeConflictEntity(
	w http.ResponseWriter, r *http.Request, m entityMutator, id conflictIdentity,
) bool {
	row := id.file
	_, err := m.AuthorizeUpdate(r.Context(), &row)
	switch {
	case err == nil:
		return true
	case errors.Is(err, entitymanager.ErrEntityNotFound):
		writeConflictNotFound(w, r)
	case writeForbiddenIfACLDenied(w, r, err):
	default:
		writeGateError(w, r, err)
	}
	return false
}

// recordConflictResolveAudit emits the audit row for a successful
// conflict resolution — the direct-file-write counterpart of the
// records entitymanager emits for manager-routed writes.
func (h *writeHandler) recordConflictResolveAudit(
	ctx context.Context, relPath string, e *entityPkg.Entity, rel *entityPkg.Relation,
) {
	op := audit.OpUpdateEntity
	if rel != nil {
		op = audit.OpUpdateRelation
	}
	h.audit().Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          op,
		Subject:     conflictAuditSubject(e, rel),
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		AssertedBy:  audit.AssertedByFrom(ctx),
		Summary:     "resolved git conflict in " + relPath,
	})
}
