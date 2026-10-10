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

	// PatchEntity is how the webhook pipeline writes: it names only the
	// properties a hook actually sets, so a property the hook does not mention
	// survives — including one redacted from the read that found the entity.
	// A read-modify-write through UpdateEntity would carry the redacted copy
	// back over the stored one. See the PatchEntity rule in CLAUDE.md.
	PatchEntity(
		ctx context.Context, id string, patch entityPkg.Patch,
	) (*entityPkg.UpdateResult, error)
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
	schema  func() *Schema
	store   store.Store
	manager entityMutator
	// softDeletes backs the Undo toast; nil when the store cannot
	// soft-delete. See softDeleter.
	softDeletes softDeleter
	reader      entityReader
	serializer  entitySerializer
	affordances affordanceService
	acl         func() acl.ACL
	audit       func() audit.Audit

	// The Lua action surface (interactive actions + webhook dispatch). All
	// three are live closures over App: the engine so fixtures that build App
	// piecemeal see late-set fields, luaDeps because the bundle is derived
	// per call from swappable collaborators, fullScriptDetail because the
	// security layer is wired after construction (SetSecurityConfig).
	engine           func() *script.Engine
	luaDeps          func() lua.WriteDeps
	fullScriptDetail func(r *http.Request) bool

	// visible is the read path's resolver. Every addressed write resolves its
	// row through it first, so a row the caller may not read, at any face, is
	// the same 404 on a write as on a GET.
	visible visibleReader

	// Shared App helpers (also used by the read path — stay on App).
	denyAfford func(
		ctx context.Context, w http.ResponseWriter, target *entityPkg.Entity, denial AffordanceDenialError,
	)
	computeETag func(ctx context.Context, e *entityPkg.Entity) string
	// faceEdges is [servedFaceEdges] bound to the App's neighbor wiring: the
	// outgoing edges of the face an entity IS, with the neighbor ids the
	// caller may see. The write path answers with the row it wrote, so it
	// owes the row's own edges, not the bare id's union of every face's.
	faceEdges func(ctx context.Context, e *entityPkg.Entity) ([]*entityPkg.Relation, map[string]bool, error)
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
// It returns the re-stamped *http.Request (not just a context) on purpose: the
// downstream read gate is consulted via helpers that take r and read
// r.Context() internally (h.visible, the serializer/reader), so the rebuilt
// ACL request + read gate must ride ON r — a bare context threaded only to the
// manager call would leave those reads on the stale, unmatched principal and
// redact the just-provisioned entity out of the response (RR-VI9XMY gap 2).
//
// contextcheck flags the resulting r.Context() reads as "non-inherited" because
// it cannot trace the derivation through the request reassignment; that whole
// class is excluded by path in .golangci.yml with the seam pinned by
// TestProvisionSeam_EveryWriteHandlerUsesWithProvision.
func (h *writeHandler) withProvision(r *http.Request) *http.Request {
	if h.provision != nil {
		return r.WithContext(h.provision(r.Context()))
	}
	return r
}

// parseCreateOpts validates a create body's ADDRESS fields — `id`, `prefix`
// and `face` — into the options the manager writes at. It writes the 422
// itself and reports ok=false when it has, so the handler branches once.
//
// entity.ParseFace is the ONLY constructor from external input (see the
// entity.Face doc), so a body-supplied face goes through it rather than a
// string conversion. Whether the type DECLARES this face is the manager's
// question, not the codec's; this only rejects what is not a face at all.
//
// An empty (or all-whitespace) face is the zero coordinate, not an error: a
// create that names no face is a different request from one that does, and the
// manager decides whether the type allows it.
// createFace picks the face a create body names, from `face` or `world`.
//
// A world names the face a create from it lands in (`worlds.<name>.create`).
// It rides the BODY, not `?world=`: the query parameter is a read-side routing
// rule that attachWorld refuses on every write, because a chain can answer
// with a FALLBACK. `create:` names one declared face directly, so resolving it
// here targets a row rather than a chain.
//
// The two spellings are exclusive rather than ranked. They can disagree, and
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
// Two refusals, both 422 and both naming the schema key the operator must fix:
// a world that is not declared, and a declared world with no `create:`. The
// second is deliberately not silent — falling back to a faceless create would
// hand the request to the manager, which refuses a faced type with
// `face_required` and names no world, leaving the operator to guess which of
// the two settings was missing.
//
// The default world is not special here: it declares no `create:` either, so a
// create from it names no face, which is exactly right for a faceless type and
// refused for a faced one.
func (h *writeHandler) createFaceForWorld(
	w http.ResponseWriter, r *http.Request, world, typeName string, def *metamodel.EntityDef,
) (string, bool) {
	// A type declaring no faces has exactly one state and no name for it, so
	// the world contributes nothing: the create is faceless whichever world it
	// came from. Returning early keeps a world-bound list working for the
	// faceless types on it, which is most of them.
	if len(def.Faces) == 0 {
		return "", true
	}
	// `default` is implicit and total, so it is never in the Worlds map
	// ([metamodel.DefaultWorldName]). It declares no `create:` and cannot: it
	// is the absence of a world, and a faced type has no default row.
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
// The PATCH path has gated this since the affordance layer landed; the create
// path did not, so an edge a `RelationVerdict{Creatable: false}` refuses on
// `POST /{plural}/{id}/relations/{rel}` was written anyway by riding the create
// body's `relations:` field. The create response even reported
// `_relations: {"<rel>": {"creatable": false}}` for the edge it had just
// written — the affordance map contradicting itself in the payload that
// produced it. Found reviewing TKT-R4BMJM, which made the path reachable from a
// button: a section create on an OUTGOING relation resolves to `link_as: to`,
// and that is exactly the direction that rides the create body.
//
// Called BEFORE CreateEntity, not beside the Phase A relation validation, which
// runs after: refusing there would leave the new entity written and unlinked,
// turning an authorization denial into an orphan. No id is needed — verdicts
// resolve against the entity's TYPE and properties, which the candidate carries.
func (h *writeHandler) gateCreateRelationAffordances(
	w http.ResponseWriter, r *http.Request,
	candidate *entityPkg.Entity, desired map[string]v1.RelationsUpdate,
) bool {
	if desired == nil {
		return true
	}
	err := h.affordances.validateRelationsModernAffordances(r.Context(), "", candidate, desired)
	return h.relationGateOK(w, r, candidate, err)
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
	// Phase A: validation only. Soft conditions surface as warnings; hard
	// wire/structural failures return immediately without applying.
	ws, err := h.validateRelationsModern(r.Context(), created.ID, created.Type, desired)
	if err != nil {
		h.writeRelationsValidationError(w, r, err)
		return nil, false
	}
	warnings = ws

	// Phase B: the writes, addressed to the face just created — it owns its
	// content-scoped edges (BUG-64MU2Q). The address comes from the row the
	// manager returned, not from the request: a create mints its id during
	// the write, so the row it produced is what names the face.
	ws, err = h.applyRelationsModern(r.Context(), created.Ref(), desired)
	warnings = append(warnings, ws...)
	if err != nil {
		h.writeRelationsApplyError(w, r, err)
		return nil, false
	}
	return warnings, true
}

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

	// Strict decode (BUG-HC6I2T): `face` used to be absent from this struct,
	// and encoding/json drops an unknown key silently — so a client naming a
	// face got 201 and a row at a face it never asked for. A misspelled key
	// must fail loudly rather than be ignored, since the request that names a
	// face and the one that names nothing now mean different things.
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

	entityDef, defOK := h.schema().Meta.Entities[typeName]
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

	// Affordance parity (BUG-Q60V): a `fields:` policy that hides or
	// freezes a field must gate it on create too, not just PATCH —
	// otherwise the value can be smuggled in at create time. Validate
	// against the candidate entity (type + proposed properties, no ID
	// yet). Relation-dependent predicates fail closed for an
	// unpersisted entity, which is the safe direction; only global-role
	// grants apply at create. Collection-level create authorization is
	// enforced separately inside CreateEntity (acl.OpCreate).
	candidate := &entityPkg.Entity{Type: typeName, Properties: req.Properties}
	if denial := h.affordances.validateFieldWrite(r.Context(), candidate, req.Properties, nil); denial != nil {
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
		// The face rides in CreateOptions, never on the carrier entity: the
		// manager authorizes and writes the value it finds there, so the two
		// cannot diverge (BUG-HC6I2T). It comes from the request body, never
		// from `?world=` — a world resolves through a chain with a fallback,
		// and a write must name the row it changes.
		createOpts,
	)
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		// A face error is about the ADDRESS, not the payload, so it gets its
		// own code: a client that omitted the face must add one, which is a
		// different fix from a property that failed validation.
		if errors.Is(err, entitymanager.ErrFaceRequired) || errors.Is(err, entitymanager.ErrFaceNotDeclared) {
			writeV1Error(w, r, http.StatusUnprocessableEntity, "face_required", err.Error(), "/face")
			return
		}
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}
	created := createResult.Entity

	relWarnings, ok := h.writeCreateRelations(w, r, created, req.Relations.Modern)
	if !ok {
		return
	}

	// The created face's own edges, not every face's (BUG-ISJHML).
	meta := h.schema().Meta
	rels := edgesOwnedBy(meta, h.reader.outgoingRelations(r.Context(), created.ID), created.Face)
	result := h.serializer.forWire(r.Context(), created, rels, meta, plural)
	if len(relWarnings) > 0 {
		result.Warnings = append(result.Warnings, relWarnings...)
	}
	// DEC-HWZHA: surface entity-level soft validation findings (e.g.
	// required-field-missing) as warnings on the 201 response.
	if len(createResult.Warnings) > 0 {
		result.Warnings = append(result.Warnings, createResult.Warnings...)
	}

	// Set Location header
	w.Header().Set("Location", fmt.Sprintf("/api/v1/%s/%s", plural, created.ID))

	// SSE broadcast is driven by the store-event bridge (see
	// App.startStoreEventBridge), not inline here — so a create by ANY process
	// reaches all connected browsers and a local create isn't double-broadcast.

	writeV1JSON(w, http.StatusCreated, result)
}

// handleV1DryRunCreate evaluates field/option/relation affordances and
// soft validation against a candidate entity WITHOUT persisting it, so
// the SPA create form can disable read-only fields, hide hidden fields,
// filter enum options, and show as-you-type validation feedback before
// commit (TKT-3I5U).
//
// It is READ-shaped (RR-R8OR): it skips the provision seam and snapshots
// state once like a GET. It is verdict-only (RR-4O6E): it computes
// affordances and warnings but emits NO `denied-write` audit row and
// performs NO write — so live re-derivation per keystroke can't flood
// the audit log.
//
// The verdicts are ADVISORY (RR-Y85M): the real create (POST without
// ?dry_run) re-runs the BUG-Q60V affordance gate and is the sole
// authorization point. A client that ignores these hints and POSTs a
// denied field still 403s.
//
// Scope: fields + options + relations + soft warnings. Relation edges
// are not staged (a candidate has no real ID); relation affordances
// reflect the per-type verdict only.
func (h *writeHandler) handleV1DryRunCreate(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	s := h.schema()

	// Mirror of handleV1CreateEntity's request body MINUS `relations`
	// — staged relations are deferred (a candidate has no real source
	// ID to hang edges on). When a new field is added to the real
	// create body, decide explicitly whether dry-run should accept it
	// and update both structs together (RR-GOR8 drift guard).
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

	// `world` resolves to a face exactly as it does on the real create, so a
	// keystroke verdict is computed against the row the submit would write.
	// A refusal here is advisory like the rest of this endpoint: it leaves
	// dryFace empty rather than answering 422, and the real POST reports it.
	rawDryFace := req.Face
	if world := strings.TrimSpace(req.World); world != "" && req.Face == "" &&
		len(entityDef.Faces) > 0 {

		if wdef, declared := s.Meta.Worlds[world]; declared {
			rawDryFace = wdef.Create
		}
	}

	// A face that is not a face at all is advisory here, like the ID warning
	// below: the create form should say so while typing rather than only at
	// submit. Whether the type declares it is ValidateCreate's question.
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

	// RR-9JOH: surface ID/prefix problems as a soft warning rather than
	// 422 so the create form learns at typing time instead of at submit.
	// The real commit's validateCreateIDOpts still hard-rejects — this
	// is advisory parity with the rest of the dry-run.
	var idWarning *Warning
	if msg := validateCreateIDOpts(&entityDef, reqID, reqPrefix); msg != "" {
		idWarning = &Warning{Code: "id_opts_invalid", Path: "/id", Detail: msg}
	}

	// Resolve the would-be entity (post template / status defaults) and
	// soft warnings via the shared create-path validation — no persist,
	// no audit, no automation. Hard structural errors surface as 422.
	candidate, warnings, err := h.manager.ValidateCreate(r.Context(),
		&entityPkg.Entity{Type: typeName, Properties: req.Properties, Content: req.Content},
		entityPkg.CreateOptions{ID: reqID, Prefix: reqPrefix, Face: dryFace},
	)
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}

	// Seed missing-but-declared property keys with nil values BEFORE
	// serialization. The SPA's create-mode field filter uses the
	// response's `properties` keys to know which declared fields are
	// visible (hidden fields get stripped by serializeEntityForWire's
	// hidden-property filter). Without this, a visible-by-default field
	// whose value the user hasn't set yet (e.g. a required `title`)
	// would be absent from both `_fields` (sparse: no deviation) and
	// `properties` (no value yet), so the filter would drop it.
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

	// Affordances are computed against the candidate's CURRENT values, so
	// value-dependent predicates (e.g. field B read-only when A == x)
	// re-derive as the form changes. includeRelations=false: no edges
	// exist for an unsaved entity.
	result := h.serializer.forWire(r.Context(), candidate, nil, h.schema().Meta, plural)
	// An unsaved candidate has nothing an action could run against, so it
	// carries no detail-action keys (TKT-VVS16W).
	maps.DeleteFunc(result.Actions, func(k string, _ bool) bool {
		return strings.HasPrefix(k, detailActionKeyPrefix)
	})
	// A create ENTERS the machine at its initial state; it is not a transition.
	// Lock every state-machine field to its entry value so the create form
	// renders it read-only at the initial state (BUG-X1C7S / TKT-3G93B8).
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

	// writeV1JSON already sets `Cache-Control: no-cache, no-store,
	// must-revalidate` and no ETag, which is what a per-request,
	// value-dependent, never-persisted response needs (RR-7PL4).
	writeV1JSON(w, http.StatusOK, result)
}

// writePatchError maps a failed PatchEntity to its HTTP status.
//
// A lost compare-and-swap race becomes a 412 — the same status the If-Match
// compare produces, because the client's remedy is identical: re-read,
// re-apply, retry. The two are genuinely the same condition detected at
// different depths (TKT-34XS2R), so giving them one status keeps the client
// contract unchanged.
//
// Without If-Match the manager retries conflicts itself, so a conflict that
// still surfaces means it ran out of retries under sustained contention.
// That is a 409: the client set no precondition, so 412 would be wrong.
//
// Matching is by errors.As, never on the message: the manager wraps the store
// error on its way up, and RR-HI9QIU is what happens when a translation
// breaks that chain — a retry loop silently becomes unreachable and every
// loser gets a 500.
func writePatchError(w http.ResponseWriter, r *http.Request, err error) {
	if writeForbiddenIfACLDenied(w, err) {
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
	writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
}

// currentVersions returns the field tokens of e as this caller sees it: the
// same face-scoped edges and redaction the GET applies.
func (h *writeHandler) currentVersions(
	ctx context.Context, e *entityPkg.Entity, plural string,
) (*v1.FieldVersions, error) {
	rels, visible, err := h.faceEdges(ctx, e)
	if err != nil {
		return nil, err
	}
	view := h.serializer.forWireScoped(ctx, e, rels, visible, h.schema().Meta, plural)
	return fieldVersionsOf(&view, h.schema().Meta), nil
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
// reports which preconditions now fail, which may be none: the other write
// can have touched only fields this request does not name. The handler does
// not retry. The client holds the edit and its base, so it decides whether to
// resend with the fresh tokens or to merge. Returns false when err is not a
// lost race. When the row is gone or now hidden from this caller it answers a
// bare 412 without tokens: the winning write may have changed what the row
// gate depends on, and a hidden row's tokens would disclose its new values.
// It is still a 412, not writePatchError's 409, because the client did set a
// precondition.
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

//nolint:gocognit,funlen // update handler threads the validation-policy classes (400/422/200-with-warnings) through each field; the branches are the documented write-policy cases, not extractable shared logic.
func (h *writeHandler) handleV1UpdateEntity(w http.ResponseWriter, r *http.Request, typeName, plural, entityID string) {
	r = h.withProvision(r)

	s := h.schema()

	// The path segment is an ADDRESS, `ID` or `ID@face`. A write names the
	// row it edits by address and never by a requested world, which is why
	// attachWorld refuses `?world=` on this method: the face rides here. A
	// bare id edits the one face the default world admits and the caller
	// may read, else 422 `face_required` (TKT-7IZHP0 §6).
	//
	// The resolver read runs BEFORE body parse, If-Match and IsLocked (RR-FGUZ,
	// RR-NGMI), so "exists but hidden" and "denied face" answer the same 404 as
	// "absent". A 400, 403, 412 or 422 here would be an existence oracle.
	entity, found := writeTargetOr404(w, r, h.visible, typeName, entityID)
	if !found {
		return
	}
	ref := entity.Ref()

	// Refuse to write through an inaccessible entity. The on-disk file
	// is unreadable (e.g. git-crypt encrypted, no key locally) — writing
	// would replace the ciphertext with whatever the SPA had on hand.
	if entity.IsLocked() {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "encrypted_inaccessible",
			"Cannot edit an inaccessible entity", "File is git-crypt encrypted; run `git-crypt unlock` first.")
		return
	}

	// Check If-Match for optimistic locking
	ifMatch := r.Header.Get("If-Match")
	if ifMatch != "" {
		currentETag := h.computeETag(r.Context(), entity)
		if ifMatch != currentETag {
			writeV1Error(w, r, http.StatusPreconditionFailed, "precondition_failed",
				"Entity has been modified", "ETag mismatch")
			return
		}
	}

	var req struct {
		Properties      map[string]any    `json:"properties,omitempty"`
		PropertiesUnset []string          `json:"properties_unset,omitempty"`
		Content         *string           `json:"content,omitempty"`
		Relations       v1.RelationsField `json:"relations"`
		Preconditions   *v1.Preconditions `json:"preconditions,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// v1.RelationsField's UnmarshalJSON returns *v1.WireError for
		// shape errors; surface them as 400 with the structured code.
		var werr *v1.WireError
		if errors.As(err, &werr) {
			writeV1Error(w, r, http.StatusBadRequest, werr.Code,
				werr.Detail, werr.Path)
			return
		}
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	// Affordance parity (TKT-G7N5): reject writes that conflict with
	// what the resolver would have surfaced on GET. Runs before any
	// other validation so the failure mode is identical regardless of
	// what else the PATCH body would have triggered.
	if denial := h.affordances.validateFieldWrite(
		r.Context(), entity, req.Properties, req.PropertiesUnset,
	); denial != nil {
		h.denyAfford(r.Context(), w, entity, *denial)
		return
	}
	if req.Relations.Modern != nil {
		// A content-scoped edge is written to the face the request
		// addresses; applyRelationsModern carries `ref` for exactly that
		// (BUG-64MU2Q). This used to refuse the write outright, because
		// entity.RelationOptions could not name a tail face and the edge
		// would have silently landed on the default one — and it advised
		// "edit it on the bare face", an address a faced type does not
		// have, so the refusal was a dead end from any client.
		err := h.affordances.validateRelationsModernAffordances(r.Context(), ref.ID, entity, req.Relations.Modern)
		if !h.relationGateOK(w, r, entity, err) {
			return
		}
	}

	// Per-field preconditions (TKT-2VDVHF). Checked after the affordance
	// gates, which refuse a hidden field, so a precondition can only name a
	// field the caller may read. The tokens are computed from this handler's
	// read, and the patch below carries that read's store version, so a write
	// landing between this check and the patch is caught by the store.
	// Relations have no store version: their check holds only under writeMu.
	scope := preconditionScope{
		props: req.Properties, unset: req.PropertiesUnset,
		content: req.Content != nil, relations: req.Relations.Modern != nil,
	}
	if !h.checkPreconditions(w, r, req.Preconditions, scope, entity, plural) {
		return
	}

	// Phase A: validate relations (no writes). Returns warnings (will
	// be merged into the success response) and err (hard 400/422).
	// Validation runs BEFORE entity update so a structural relation
	// error doesn't leave the entity half-written. (DEC-HWZHA atomicity.)
	var warnings []Warning
	if req.Relations.Modern != nil {
		ws, err := h.validateRelationsModern(r.Context(), ref.ID, entity.Type, req.Relations.Modern)
		if err != nil {
			h.writeRelationsValidationError(w, r, err)
			return
		}
		warnings = ws
	}

	// Phase B: entity update. Skipped when only relations changed,
	// to avoid bumping the file mtime and broadcasting a misleading
	// "entity updated" SSE event with no byte-level change.
	//
	// Warn about undeclared unset keys before dispatching. The unset itself
	// is carried by the patch (MetaUnset), applied AFTER the property
	// upserts, so a body that both sets and unsets the same key ends with it
	// removed. (TKT-E6094 / autosave: maps the "user cleared this field"
	// intent to a wire-level delete distinct from "field was untouched".)
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
	entityChanged := req.Properties != nil || len(req.PropertiesUnset) > 0 || req.Content != nil
	if entityChanged {
		// PatchEntity + ExpectedVersion, NOT a read-modify-write into
		// UpdateEntity (TKT-34XS2R). Two things change:
		//
		//  1. Properties this request does not name are preserved by the
		//     manager's own merge against the raw stored entity, so a
		//     redacted read can no longer erase what it could not see.
		//  2. The If-Match check is not a check-then-write. Comparing the
		//     header to a freshly-read ETag and then writing would race any
		//     concurrent writer; the store re-verifies the precondition
		//     ATOMICALLY with the write, so a racing writer that lands between
		//     our read and our write is caught rather than silently
		//     overwritten.
		//
		// The ETag compare above stays: it is the client-facing contract and
		// is relation-aware, which the store token deliberately is not.
		patch := entityPkg.Patch{
			Properties:      req.Properties,
			MetaUnset:       req.PropertiesUnset,
			Content:         req.Content,
			ExpectedVersion: string(store.VersionOf(entity)),
		}
		// Fused ref, not the bare id: PatchEntity resolves the face from the
		// STORED row it loads, and loading by bare id would land on the
		// default face — so a write to POL-1@published would be authorized
		// (and applied) against POL-1's default state instead.
		stateRef := entityPkg.FormatStateRef(entity.ID, entity.Face)
		updateResult, err := h.manager.PatchEntity(r.Context(), stateRef, patch)
		if err != nil {
			if req.Preconditions != nil && h.writeLostRace(w, r, err, req.Preconditions, typeName, ref, plural) {
				return
			}
			writePatchError(w, r, err)
			return
		}
		// PatchEntity merged against the raw stored entity; refresh the
		// local copy so the response body and ETag reflect what was actually
		// persisted rather than the pre-merge read.
		if updateResult != nil && updateResult.Entity != nil {
			entity = updateResult.Entity
		}
		// DEC-HWZHA: soft validation findings ride on the result as
		// warnings. Merge them into the response alongside any
		// relation warnings already collected.
		if updateResult != nil {
			warnings = append(warnings, updateResult.Warnings...)
		}
	}

	// Phase C: relation writes. Produces warnings on soft conditions
	// and structured errors on hard failures.
	if req.Relations.Modern != nil {
		ws, err := h.applyRelationsModern(r.Context(), ref, req.Relations.Modern)
		warnings = append(warnings, ws...)
		if err != nil {
			h.writeRelationsApplyError(w, r, err)
			return
		}
	}

	// The edges OF THE FACE JUST WRITTEN, through the same face-scoped seam
	// the read surfaces use. The bare-id reader returns the UNION of every
	// face's content-scoped edges, so a PATCH to the published face would
	// answer with the draft's links beside it — the mixed-face response the
	// seam exists to prevent, on a body the SPA feeds straight back into its
	// relation editor.
	rels, visibleNeighbors, rerr := h.faceEdges(r.Context(), entity)
	if rerr != nil {
		writeGateError(w, r, rerr)
		return
	}
	// The response describes the STORED row, not the entity the manager
	// handed back: fsstore reformats the body on write, so tokens of the
	// in-memory body would never match the next GET and every later save
	// would 412. Body and tokens come from this one read, so they describe one
	// state even when another node wrote in between. The read is gated like a
	// GET: a row the write (or a concurrent one) hid from this caller answers
	// with the written entity and no tokens.
	served, versioned := h.visibleStored(r.Context(), typeName, ref)
	if !versioned {
		served = entity
	}
	result := h.serializer.forWireScoped(r.Context(), served, rels, visibleNeighbors, h.schema().Meta, plural)
	if len(warnings) > 0 {
		result.Warnings = warnings
	}
	if versioned {
		result.Versions = fieldVersionsOf(&result, h.schema().Meta)
	}
	newETag := h.computeETag(r.Context(), served)
	w.Header().Set("ETag", newETag)

	// SSE broadcast is driven by the store-event bridge: an entity update only
	// fires EventEntityUpdated when the store's entity row actually changed,
	// which matches the prior "if entityChanged" gate (relation-only edits emit
	// no entity event). So a remote update reaches all browsers and a local one
	// isn't double-broadcast.

	writeV1JSON(w, http.StatusOK, result)
}

func (h *writeHandler) handleV1DeleteEntity(w http.ResponseWriter, r *http.Request, typeName, _, entityID string) {
	r = h.withProvision(r)

	// The path segment is an ADDRESS, resolved as every content write is
	// ([writeTargetOr404]). On a type that declares faces, a delete names one
	// face (`ID@face`) and removes that face; the last face takes the entity
	// with it. A bare id there is `face_required`: deleting every face would
	// need delete on faces the caller may not see. A faceless type's bare id
	// names its one face, the implicit one, and deletes the entity.
	//
	// The resolver read runs BEFORE AuthorizeWrite (RR-3532), so a hidden
	// target or a denied face 404s rather than answering 403-with-rule_id.
	entity, found := writeTargetOr404(w, r, h.visible, typeName, entityID)
	if !found {
		return
	}
	ref := entity.Ref()

	var err error
	if ref.Face.IsImplicit() {
		err = h.deleteWholeEntity(r.Context(), ref.ID)
	} else {
		_, err = h.manager.DeleteEntityFace(r.Context(), ref.ID, ref.Face, true)
	}
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		// Another request deleted it between the read above and this write.
		if errors.Is(err, entitymanager.ErrEntityNotFound) {
			writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
			return
		}
		writeInternalError(w, r, "delete_failed", "Failed to delete entity", err)
		return
	}

	// SSE broadcast is driven by the store-event bridge (see
	// App.startStoreEventBridge); a delete by any process reaches all browsers,
	// and a local delete isn't double-broadcast.

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
	if writeForbiddenIfACLDenied(w, err) {
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
	writeInternalErrorDetail(w, r, "relation_write_failed",
		"Failed to apply relation changes after entity update; the entity may have been updated",
		detail+internalErrorDetail, err)
}

func (h *writeHandler) handleV1CreateRelation(
	w http.ResponseWriter, r *http.Request, typeName, addr, relType string,
) {
	r = h.withProvision(r)

	// ACL gate (TKT-VQGN CRIT-2): runs BEFORE body parse (RR-FGUZ applied to
	// relation writes) and BEFORE the affordance check, otherwise a 400/403
	// confirms the entity exists. The resolver applies the face gate too, so
	// a face the caller may not read is the same 404 (BUG-BZQQDP).
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

	// Read-gate the BODY target as well as the path entity.
	//
	// The path gate above is not sufficient, and on this route it can be
	// vacuous: a client that just created the path entity is guaranteed to pass
	// it. Without this, naming an unreadable entity as the target returned 201
	// while a nonexistent one returned 422 "target entity not found" — an
	// existence oracle for any id the caller cannot read, and a link to a row
	// they were never shown.
	//
	// Pre-existing (the body target was never gated), but TKT-R4BMJM put weight
	// on it: the `link_as: from` create is now addressed FROM the new entity
	// with the peer in the body, precisely so a prefix-less peer id can be
	// linked. That moved the peer out from under the path gate.
	//
	// The target's type comes from the STORE, not from its id prefix: a
	// prefix-derived type is exactly the fragile lookup that made a
	// legitimately prefix-less id unlinkable. The check is the entity-level
	// one: some face of the peer the caller may read. A target that does not
	// exist and one the caller may not read collapse to the same 404 carrying
	// the shared entityNotFoundTitle, so the two stay indistinguishable.
	// Whether an entity exists is a genuine secret (docs/acl-security.md).
	peer, ok := h.bodyPeer(w, r, relType, req.ID, req.Direction)
	if !ok {
		return
	}

	// The new edge's SOURCE and TAIL: the path entity by ownedSource's rule
	// on an outgoing edge, the peer the body names on an incoming one.
	newTail := peer.Face
	if req.Direction != string(DirectionIncoming) {
		if entity, newTail, ok = h.ownedSource(w, r, typeName, addr, relType, entity); !ok {
			return
		}
	}

	// Affordance gates: creatable + meta-writable, evaluated against
	// the SOURCE of the new edge (not necessarily the path entity —
	// for incoming-direction creates the path entity is the target).
	sources, ok := h.relationSourcesOr500(w, r, entity, peer, req.Direction, relType)
	if !ok {
		return
	}
	// Audit subject is the source row whose policy denied the write.
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
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusUnprocessableEntity, "relation_failed", "Failed to create relation", err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func (h *writeHandler) handleV1UpdateRelation(
	w http.ResponseWriter, r *http.Request, typeName, addr, relType, targetID string,
) {
	r = h.withProvision(r)

	// ACL gate (TKT-VQGN CRIT-2): see handleV1CreateRelation. The peer is
	// gated too, before the edge or its affordances are touched, so a hidden
	// peer is the same 404 as an absent one (design 8.2).
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

	// Affordance gate: meta-writable, evaluated against the SOURCE of
	// the edge (the path entity for outgoing; the peer for incoming).
	// The edge already exists (PATCH is meta-only), so the create /
	// remove gates don't apply.
	peer := peerAddress(targetID, req.Direction == string(DirectionIncoming))
	from, to := resolveRelationEndpoints(entity.ID, peer.ID(), req.Direction)

	// The edge's source row and tail: by ownedSource's rule when the path
	// entity is the source, else the stored edge's own tail. The affordance
	// gate reads the source at this tail.
	entity, tail, ok := h.edgeSource(w, r, typeName, addr, entity, peer, from, relType, to)
	if !ok {
		return
	}

	sources, ok := h.relationSourcesOr500(w, r, entity, entityPkg.Ref{ID: from, Face: tail}, req.Direction, relType)
	if !ok {
		return
	}
	if req.Position != nil {
		writeRelationPosition(w, r, h, relationPositionRequest{
			key:       entityPkg.RelationKey{From: from, FromFace: tail, Type: relType, To: to},
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

	// Managed order properties must be finite numbers when present. Fast
	// 400 here so wire-format errors don't surface as 422-from-manager.
	if relDef, ok := h.schema().Meta.Relations[relType]; ok {
		for _, prop := range []string{metamodel.OrderPropertyOut, metamodel.OrderPropertyIn} {
			if (prop == metamodel.OrderPropertyOut && relDef.OutgoingOrderProperty() == "") ||
				(prop == metamodel.OrderPropertyIn && relDef.IncomingOrderProperty() == "") {

				continue
			}
			v, present := req.Meta[prop]
			if !present {
				continue
			}
			if _, ok := entitymanager.FiniteOrder(v); !ok {
				writeV1Error(w, r, http.StatusBadRequest, "order_value_invalid",
					"managed order property must be a finite number", prop)
				return
			}
		}
	}

	rel, err := h.manager.UpdateRelation(r.Context(),
		entityPkg.RelationKey{From: from, FromFace: tail, Type: relType, To: to},
		entityPkg.RelationOptions{Properties: req.Meta})
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusNotFound, "relation_not_found", "Relation not found", err.Error())
		return
	}

	result := map[string]any{
		"from": rel.From,
		"type": rel.Type,
		"to":   rel.To,
	}
	if len(rel.Properties) > 0 {
		result["meta"] = rel.Properties
	}

	writeV1JSON(w, http.StatusOK, result)
}

func (h *writeHandler) handleV1DeleteRelation(
	w http.ResponseWriter, r *http.Request, typeName, addr, relType, targetID string,
) {
	r = h.withProvision(r)

	// ACL gate (TKT-VQGN CRIT-2): see handleV1CreateRelation. The peer is
	// gated too, before the edge or its affordances are touched, so a hidden
	// peer is the same 404 as an absent one (design 8.2).
	entity, found := readAddressedOr404(w, r, h.visible, typeName, addr)
	if !found {
		return
	}
	if !familyReadableOr404(w, r, h.visible, relationTargetID(targetID)) {
		return
	}

	// Affordance gate: removable check, evaluated against the SOURCE
	// of the edge (the path entity for outgoing; the peer for
	// incoming). Per-relation-type uniform — a removable=false
	// verdict applies to every link of this type.
	direction := r.URL.Query().Get("direction")
	peer := peerAddress(targetID, direction == string(DirectionIncoming))
	from, to := resolveRelationEndpoints(entity.ID, peer.ID(), direction)

	// Addressed by its OWN tail — see incomingEdgeTail. Dropping the tail
	// deletes a DIFFERENT edge (the default face's) and reports success.
	// DeleteRelationState with the zero face IS DeleteRelation, so the
	// faceless and identity-scoped cases are unchanged. The affordance gate
	// reads the source at this tail too.
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

	if err := h.manager.DeleteRelation(r.Context(),
		entityPkg.RelationKey{From: from, FromFace: tail, Type: relType, To: to}); err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusNotFound, "relation_not_found", "Relation not found", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *writeHandler) handleV1CloneEntity(
	w http.ResponseWriter, r *http.Request, typeName, addr string,
) {
	r = h.withProvision(r)

	s := h.schema()

	// ACL gate (TKT-VQGN): a clone from a hidden source, or from a face the
	// caller may not read, 404s like a clone from a nonexistent source.
	entity, found := readAddressedOr404(w, r, h.visible, typeName, addr)
	if !found {
		return
	}

	// Clone properties. File values stay behind: they name the source's
	// bytes, which the clone does not have, and only the attachments API
	// may set a file value (BUG-CTUW2N).
	props := make(map[string]any)
	maps.Copy(props, entity.Properties)
	for _, prop := range metamodel.FileProperties(s.Meta, typeName) {
		delete(props, prop)
	}

	// The clone lands on the SOURCE's face, not the bare coordinate
	// (BUG-HC6I2T): a clone of a draft is a draft. The source face is read
	// off the row that was actually loaded rather than re-derived, so the
	// copy and its original cannot end up at different coordinates.
	cloneResult, err := h.manager.CreateEntity(r.Context(),
		&entityPkg.Entity{
			Type:       typeName,
			Properties: props,
			Content:    entity.Content,
		},
		entityPkg.CreateOptions{Face: entity.Face},
	)
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		// The clone is a create, so it fails validation like one: a
		// `unique:` property always collides with its source.
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

// handleV1ConflictResolve applies a conflict resolution.
func (h *writeHandler) handleV1ConflictResolve(w http.ResponseWriter, r *http.Request) {
	var req v1.ConflictResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_request", "Invalid JSON", err.Error())
		return
	}

	if req.Path == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_path", "Path is required", "")
		return
	}

	// The path is caller-supplied — contain it to the project root
	// before any filesystem access.
	absPath, ok := resolveConflictPath(h.paths, w, r, req.Path)
	if !ok {
		return
	}

	r = h.withProvision(r)

	st := h.schema()

	cf, err := conflict.ParseConflictedFile(absPath, st.Meta)
	if err != nil {
		writeInternalError(w, r, "parse_failed", "Failed to parse conflict", err)
		return
	}

	resolution := &conflict.Resolution{
		PropertyChoices: make(map[string]conflict.Side),
	}

	// Map property choices
	for prop, choice := range req.PropertyChoices {
		if choice == "theirs" {
			resolution.PropertyChoices[prop] = conflict.SideTheirs
		} else {
			resolution.PropertyChoices[prop] = conflict.SideOurs
		}
	}

	// Map content choice
	switch req.ContentChoice {
	case "theirs":
		resolution.ContentChoice = conflict.SideTheirs
	case "manual":
		resolution.ManualContent = req.ManualContent
	default:
		resolution.ContentChoice = conflict.SideOurs
	}

	// Resolve first so the ACL gate evaluates the actual write target
	// (entity vs relation, post-choice identity), then authorize, then
	// write. The write is file-level marker removal and cannot route
	// through entitymanager — the store can't parse a file that still
	// contains conflict markers — so this handler re-authorizes and
	// audits explicitly. The store's file watcher picks the change up
	// as an external edit, keeping index/SSE consumers in sync.
	resolvedEntity, resolvedRelation, err := conflict.Resolve(cf, resolution)
	if err != nil {
		// Resolve fails only when the two sides are not both parseable
		// entities or both relations: a fact about the file, not the server.
		writeV1Error(w, r, http.StatusUnprocessableEntity, "conflict_unresolvable",
			"The conflict cannot be resolved automatically", "Resolve it by editing the file")
		return
	}
	if !h.authorizeConflictResolve(r.Context(), w, resolvedEntity, resolvedRelation) {
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

// authorizeConflictResolve re-authorizes the write a conflict
// resolution performs. Conflict resolution bypasses entitymanager, so
// the gate the manager would normally apply lives here: entity files
// gate like an entity update; relation files gate like a relation
// update (source-entity type, mirroring entitymanager.UpdateRelation —
// type left empty when the source entity can't be loaded, the same
// fallback the manager uses). A deny records a `denied-write` audit
// row and writes the standard 403 body; returns true when the write
// may proceed.
func (h *writeHandler) authorizeConflictResolve(
	ctx context.Context, w http.ResponseWriter, e *entityPkg.Entity, rel *entityPkg.Relation,
) bool {
	var aclReq acl.WriteRequest
	if rel != nil {
		// Write authorization needs the real type whether or not the caller
		// may read the row, and a faced source has no zero-face row. The
		// conflict parser reads no tail, so an edge from a faced source is
		// judged on every face its type declares: at least what the manager
		// asks for the edge at any tail.
		fromType := storedTypeOf(ctx, h.store, rel.From)
		aclReq = translateRelationWrite(h.schema().Meta, rel.Type, fromType, rel.From, rel.FromFace)
	} else {
		aclReq = translateVerb("update", e.Type, e.ID, e.Face)
	}
	decision := h.acl().AuthorizeWrite(ctx, aclReq)
	if decision.Allow {
		return true
	}
	h.audit().Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpDeniedWrite,
		Subject:     conflictAuditSubject(e, rel),
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary: fmt.Sprintf("denied: %s (rule_kind=%s rule_id=%s op=conflict-resolve)",
			decision.Reason, decision.RuleKind, decision.RuleID),
	})
	writeForbiddenIfACLDenied(w, &acl.ForbiddenError{Decision: decision})
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
		Summary:     "resolved git conflict in " + relPath,
	})
}
