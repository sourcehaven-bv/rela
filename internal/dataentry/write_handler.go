package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"strings"
	"sync"
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
	DeleteEntityFace(ctx context.Context, id string, face entityPkg.Face) (*entityPkg.DeleteResult, error)
	CreateRelation(
		ctx context.Context, from, relType, to string, opts entityPkg.RelationOptions,
	) (*entityPkg.Relation, error)
	UpdateRelation(
		ctx context.Context, from, relType, to string, opts entityPkg.RelationOptions,
	) (*entityPkg.Relation, error)
	DeleteRelation(ctx context.Context, from, relType, to string) error

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
// passed as closures (gateRead, denyAfford, computeETag, currentEdgesByPeer)
// so the two paths cannot drift (uniform-404 read gate, affordance-denial
// audit, one ETag definition).
//
// writeMu is a POINTER to App's mutation mutex: every handler here serializes
// against the extracted sync/attachment handlers, exactly as before. With the
// action surface moved in, writeHandler covers the complete data-entry write
// surface — no App method takes writeMu directly anymore. (The DEC-8UIL0 arc
// later replaces this mutex with the store's Tx contract; that is deliberately
// NOT part of this refactor.)
type writeHandler struct {
	schema      func() *Schema
	store       store.Store
	manager     entityMutator
	reader      entityReader
	serializer  entitySerializer
	affordances affordanceService
	acl         func() acl.ACL
	audit       func() audit.Audit

	// The Lua action surface (interactive actions + webhook dispatch) —
	// scripts may mutate the workspace, so they run under writeMu. All three
	// are live closures over App: the engine so fixtures that build App
	// piecemeal see late-set fields, luaDeps because the bundle is derived
	// per call from swappable collaborators, fullScriptDetail because the
	// security layer is wired after construction (SetSecurityConfig).
	engine           func() *script.Engine
	luaDeps          func() lua.WriteDeps
	fullScriptDetail func(r *http.Request) bool

	// Shared App helpers (also used by the read path — stay on App).
	gateRead   func(w http.ResponseWriter, r *http.Request, typeName, entityID string) bool
	denyAfford func(
		ctx context.Context, w http.ResponseWriter, target *entityPkg.Entity, denial AffordanceDenialError,
	)
	computeETag        func(ctx context.Context, e *entityPkg.Entity) string
	currentEdgesByPeer func(
		ctx context.Context, entityID, canonical string, incoming bool,
	) map[string]*entityPkg.Relation

	// paths contains caller-supplied conflict-file paths to the project
	// root (conflict-resolve is the one file-level write in the nucleus).
	paths *project.Context

	// provision implements unmatched_principal: provision (TKT-ANUJDS). Called
	// at the top of each write handler, under the writeMu it already holds; it
	// lazily creates a stub user entity for an unmatched verified principal and
	// returns a ctx re-stamped to it (with a rebuilt ACL request + read gate).
	// The handler adopts the returned ctx. A no-op on every non-provision write.
	provision func(context.Context) context.Context

	writeMu *sync.Mutex
}

// enterWrite acquires the write lock and runs the provision seam under it,
// returning the CONTEXT the handler must thread downstream — re-stamped to a
// freshly-provisioned entity when unmatched_principal: provision fired, else the
// request's own context unchanged. The caller MUST `defer h.writeMu.Unlock()`
// itself — the unlock cannot live here because it must span the whole handler
// body. Running provision under the lock already held serializes the stub create
// against every other mutation for free (see maybeProvision).
//
// It returns the re-stamped *http.Request (not just a context) on purpose: the
// downstream read gate is consulted via helpers that take r and read
// r.Context() internally (h.gateRead, the serializer/reader), so the rebuilt
// ACL request + read gate must ride ON r — a bare context threaded only to the
// manager call would leave those reads on the stale, unmatched principal and
// redact the just-provisioned entity out of the response (RR-VI9XMY gap 2).
//
// contextcheck flags the resulting r.Context() reads as "non-inherited" because
// it cannot trace the derivation through the request reassignment; that whole
// class is excluded by path in .golangci.yml with the seam pinned by
// TestProvisionSeam_EveryWriteHandlerUsesEnterWrite.
func (h *writeHandler) enterWrite(r *http.Request) *http.Request {
	h.writeMu.Lock()
	if h.provision != nil {
		return r.WithContext(h.provision(r.Context()))
	}
	return r
}

func (h *writeHandler) handleV1CreateEntity(w http.ResponseWriter, r *http.Request, typeName, plural string) {
	// Need write lock for creation
	r = h.enterWrite(r)
	defer h.writeMu.Unlock()

	var req struct {
		ID         string            `json:"id,omitempty"`
		Prefix     string            `json:"prefix,omitempty"`
		Properties map[string]any    `json:"properties"`
		Content    string            `json:"content,omitempty"`
		Relations  v1.RelationsField `json:"relations"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
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

	req.ID = strings.TrimSpace(req.ID)
	req.Prefix = strings.TrimSpace(req.Prefix)
	if msg := validateCreateIDOpts(&entityDef, req.ID, req.Prefix); msg != "" {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", msg, "")
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

	createResult, err := h.manager.CreateEntity(r.Context(),
		&entityPkg.Entity{
			Type:       typeName,
			Properties: req.Properties,
			Content:    req.Content,
		},
		entityPkg.CreateOptions{ID: req.ID, Prefix: req.Prefix},
	)
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
		return
	}
	created := createResult.Entity

	// Phase A: relation validation (mirrors the PATCH path). Soft
	// conditions surface as warnings; hard wire/structural failures
	// return immediately without applying.
	var relWarnings []Warning
	if req.Relations.Modern != nil {
		ws, err := h.validateRelationsModern(r.Context(), created.ID, created.Type, req.Relations.Modern)
		if err != nil {
			h.writeRelationsValidationError(w, r, err)
			return
		}
		relWarnings = ws
	}

	// Phase B: relation writes.
	if req.Relations.Modern != nil {
		ws, err := h.applyRelationsModern(r.Context(), created.ID, req.Relations.Modern)
		relWarnings = append(relWarnings, ws...)
		if err != nil {
			h.writeRelationsApplyError(w, r, err)
			return
		}
	}

	rels := h.reader.outgoingRelations(r.Context(), created.ID)
	result := h.serializer.forWire(r.Context(), created, rels, h.schema().Meta, plural)
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
// It is READ-shaped (RR-R8OR): it never takes h.writeMu and snapshots
// state once like a GET. It is verdict-only (RR-4O6E): it computes
// affordances and warnings but emits NO `denied-write` audit row and
// performs NO write — so live re-derivation per keystroke can't flood
// the audit log or contend the writer lock.
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
		entityPkg.CreateOptions{ID: reqID, Prefix: reqPrefix},
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
	// A create ENTERS the machine at its initial state; it is not a transition.
	// Lock every state-machine field to its entry value so the create form
	// renders it read-only at the initial state (BUG-X1C7S / TKT-3G93B8).
	h.serializer.affordances.applyCreateLock(r.Context(), &result, candidate)
	if idWarning != nil {
		result.Warnings = append(result.Warnings, *idWarning)
	}
	if len(warnings) > 0 {
		result.Warnings = append(result.Warnings, warnings...)
	}

	// writeV1JSON already sets `Cache-Control: no-cache, no-store,
	// must-revalidate` and no ETag, which is what a per-request,
	// value-dependent, never-persisted response needs (RR-7PL4).
	writeV1JSON(w, http.StatusOK, result)
}

//nolint:gocognit,funlen // update handler threads the validation-policy classes (400/422/200-with-warnings) through each field; the branches are the documented write-policy cases, not extractable shared logic.
func (h *writeHandler) handleV1UpdateEntity(w http.ResponseWriter, r *http.Request, typeName, plural, entityID string) {
	// Need write lock
	r = h.enterWrite(r)
	defer h.writeMu.Unlock()

	s := h.schema()

	// The path segment is an ADDRESS — `ID` or `ID@face` (see entityRef). A
	// write names the row it edits by address and never by world, which is
	// why attachWorld refuses `?world=` on this method: the face rides here.
	ref, ok := parseEntityRef(s.Meta, typeName, entityID)
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// ACL gate (TKT-VQGN): runs BEFORE getEntity (RR-NGMI timing) AND
	// before body parse / If-Match / IsLocked so the only observable
	// for "this id exists but you can't see it" is the same 404 as
	// "this id doesn't exist" (RR-FGUZ). A 400 / 412 / 422 here would
	// be an existence oracle. On the BARE id: the row gate is face-blind.
	if !h.gateRead(w, r, typeName, ref.ID) {
		return
	}

	entity, found := h.reader.getEntityRef(r.Context(), ref)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

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
		// A content-scoped relation type attaches to ONE face on its tail
		// side, and the relation writers below address the entity's bare
		// tail (entity.RelationOptions carries no face). Writing such an
		// edge from a non-bare face would therefore silently attach it to
		// the bare face — the wrong-face write this whole address grammar
		// exists to prevent. Identity-scoped edges are entity-level and
		// write the same edge from every face, so they pass through.
		if relType, refused := contentScopedRelationOn(s.Meta, ref, req.Relations.Modern); refused {
			writeV1Error(w, r, http.StatusUnprocessableEntity, "face_relations_unsupported",
				"Content-scoped relations cannot be written through a face address",
				fmt.Sprintf("relation %q is `scope: content`; edit it on the bare face, "+
					"or move it with a copy definition", relType))
			return
		}
		if denial := h.affordances.validateRelationsModernAffordances(
			r.Context(), ref.ID, entity, req.Relations.Modern,
		); denial != nil {
			h.denyAfford(r.Context(), w, entity, *denial)
			return
		}
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
	if req.Properties != nil {
		maps.Copy(entity.Properties, req.Properties)
	}
	// Apply properties_unset AFTER property upserts so a body that
	// both sets and unsets the same key behaves like the last-write-
	// wins of property merging followed by the explicit unset.
	// (TKT-E6094 / autosave: maps the "user cleared this field" intent
	// to a wire-level delete that's distinct from "field was untouched".)
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
			delete(entity.Properties, k)
		}
	}
	if req.Content != nil {
		entity.Content = *req.Content
	}
	entityChanged := req.Properties != nil || len(req.PropertiesUnset) > 0 || req.Content != nil
	if entityChanged {
		updateResult, err := h.manager.UpdateEntity(r.Context(), entity)
		if err != nil {
			if writeForbiddenIfACLDenied(w, err) {
				return
			}
			writeV1Error(w, r, http.StatusUnprocessableEntity, "validation_failed", "Validation failed", err.Error())
			return
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
		ws, err := h.applyRelationsModern(r.Context(), ref.ID, req.Relations.Modern)
		warnings = append(warnings, ws...)
		if err != nil {
			h.writeRelationsApplyError(w, r, err)
			return
		}
	}

	rels := h.reader.outgoingRelations(r.Context(), entity.ID)
	result := h.serializer.forWire(r.Context(), entity, rels, h.schema().Meta, plural)
	if len(warnings) > 0 {
		result.Warnings = warnings
	}
	newETag := h.computeETag(r.Context(), entity)
	w.Header().Set("ETag", newETag)

	// SSE broadcast is driven by the store-event bridge: an entity update only
	// fires EventEntityUpdated when the store's entity row actually changed,
	// which matches the prior "if entityChanged" gate (relation-only edits emit
	// no entity event). So a remote update reaches all browsers and a local one
	// isn't double-broadcast.

	writeV1JSON(w, http.StatusOK, result)
}

func (h *writeHandler) handleV1DeleteEntity(w http.ResponseWriter, r *http.Request, typeName, _, entityID string) {
	// Need write lock
	r = h.enterWrite(r)
	defer h.writeMu.Unlock()

	// The path segment is an ADDRESS (see entityRef). `ID` and `ID@<bare>`
	// delete the whole entity; `ID@face` for a non-bare face deletes THAT
	// face only — the "unpublish" the address grammar makes expressible.
	ref, ok := parseEntityRef(h.schema().Meta, typeName, entityID)
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// ACL gate (TKT-VQGN): runs BEFORE getEntity (RR-NGMI timing) AND
	// before AuthorizeWrite (RR-3532 — so a hidden target 404s, not
	// 403-with-rule_id). On the BARE id: the row gate is face-blind.
	if !h.gateRead(w, r, typeName, ref.ID) {
		return
	}

	entity, found := h.reader.getEntityRef(r.Context(), ref)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	var err error
	if ref.Face.IsDefault() {
		_, err = h.manager.DeleteEntity(r.Context(), ref.ID, true)
	} else {
		_, err = h.manager.DeleteEntityFace(r.Context(), ref.ID, ref.Face)
	}
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusInternalServerError, "delete_failed", "Failed to delete entity", err.Error())
		return
	}

	// SSE broadcast is driven by the store-event bridge (see
	// App.startStoreEventBridge); a delete by any process reaches all browsers,
	// and a local delete isn't double-broadcast.

	w.WriteHeader(http.StatusNoContent)
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
	writeV1Error(w, r, http.StatusUnprocessableEntity,
		"relation_failed", "Failed to validate relations", err.Error())
}

// writeRelationsApplyError maps a Phase C write error to a 500 — the
// entity may already have been updated, so a partial state is on disk.
// This is the documented atomicity gap. ACL denials short-circuit to
// the structured 403 path; a dangling-peer structuralError maps to 422
// (the reference did not resolve, so the edge was not stored —
// BUG-K6FEVB); everything else falls through to the 500-with-detail body.
func (h *writeHandler) writeRelationsApplyError(w http.ResponseWriter, r *http.Request, err error) {
	if writeForbiddenIfACLDenied(w, err) {
		return
	}
	if se, ok := asStructuralError(err); ok {
		writeV1Error(w, r, http.StatusUnprocessableEntity, se.Code, se.Detail, se.Path)
		return
	}
	writeV1Error(w, r, http.StatusInternalServerError,
		"relation_write_failed",
		"Failed to apply relation changes after entity update; the entity may have been updated",
		reconcileDetail(err))
}

func (h *writeHandler) handleV1CreateRelation(
	w http.ResponseWriter, r *http.Request, typeName, entityID, relType string,
) {
	// Need write lock
	r = h.enterWrite(r)
	defer h.writeMu.Unlock()

	// ACL gate (TKT-VQGN CRIT-2): runs BEFORE body parse (RR-FGUZ
	// applied to relation writes) and BEFORE the affordance check —
	// otherwise a 400/403 confirms the entity exists.
	if !h.gateRead(w, r, typeName, entityID) {
		return
	}

	entity, found := h.reader.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
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

	// Affordance gates: creatable + meta-writable, evaluated against
	// the SOURCE of the new edge (not necessarily the path entity —
	// for incoming-direction creates the path entity is the target).
	source := h.affordances.relationSourceEntity(r.Context(), entity, req.ID, req.Direction)
	// Audit subject is the source of the new edge, matching the
	// entity whose policy gated the write.
	if denial := h.affordances.validateRelationOp(r.Context(), source, relType, RelationOpCreate); denial != nil {
		h.denyAfford(r.Context(), w, source, *denial)
		return
	}
	if denial := h.affordances.validateRelationMetaWrite(r.Context(), source, relType, req.Meta, nil); denial != nil {
		h.denyAfford(r.Context(), w, source, *denial)
		return
	}

	from, to := resolveRelationEndpoints(entity.ID, req.ID, req.Direction)

	_, err := h.manager.CreateRelation(
		r.Context(), from, relType, to, entityPkg.RelationOptions{Properties: req.Meta},
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
	w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string,
) {
	// Need write lock
	r = h.enterWrite(r)
	defer h.writeMu.Unlock()

	// ACL gate (TKT-VQGN CRIT-2): see handleV1CreateRelation.
	if !h.gateRead(w, r, typeName, entityID) {
		return
	}

	entity, found := h.reader.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	var req struct {
		Meta      map[string]any `json:"meta"`
		Direction string         `json:"direction,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_json", "Invalid JSON body", err.Error())
		return
	}

	// Affordance gate: meta-writable, evaluated against the SOURCE of
	// the edge (the path entity for outgoing; the peer for incoming).
	// The edge already exists (PATCH is meta-only), so the create /
	// remove gates don't apply.
	source := h.affordances.relationSourceEntity(r.Context(), entity, targetID, req.Direction)
	if denial := h.affordances.validateRelationMetaWrite(r.Context(), source, relType, req.Meta, nil); denial != nil {
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

	from, to := resolveRelationEndpoints(entity.ID, targetID, req.Direction)

	rel, err := h.manager.UpdateRelation(r.Context(), from, relType, to, entityPkg.RelationOptions{
		Properties: req.Meta,
	})
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
	w http.ResponseWriter, r *http.Request, typeName, entityID, relType, targetID string,
) {
	// Need write lock
	r = h.enterWrite(r)
	defer h.writeMu.Unlock()

	// ACL gate (TKT-VQGN CRIT-2): see handleV1CreateRelation.
	if !h.gateRead(w, r, typeName, entityID) {
		return
	}

	entity, found := h.reader.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// Affordance gate: removable check, evaluated against the SOURCE
	// of the edge (the path entity for outgoing; the peer for
	// incoming). Per-relation-type uniform — a removable=false
	// verdict applies to every link of this type.
	direction := r.URL.Query().Get("direction")
	source := h.affordances.relationSourceEntity(r.Context(), entity, targetID, direction)
	if denial := h.affordances.validateRelationOp(r.Context(), source, relType, RelationOpRemove); denial != nil {
		h.denyAfford(r.Context(), w, source, *denial)
		return
	}

	from, to := resolveRelationEndpoints(entity.ID, targetID, direction)

	if err := h.manager.DeleteRelation(r.Context(), from, relType, to); err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusNotFound, "relation_not_found", "Relation not found", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *writeHandler) handleV1CloneEntity(w http.ResponseWriter, r *http.Request, typeName, entityID string) {
	// Need write lock
	r = h.enterWrite(r)
	defer h.writeMu.Unlock()

	s := h.schema()

	// ACL gate (TKT-VQGN): runs BEFORE getEntity (RR-NGMI timing) so
	// a clone from a hidden source 404s with the same shape and
	// timing as a clone from a nonexistent source.
	if !h.gateRead(w, r, typeName, entityID) {
		return
	}

	entity, found := h.reader.getEntity(r.Context(), entityID)
	if !found || entity.Type != typeName {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Entity not found", "")
		return
	}

	// Clone properties
	props := make(map[string]any)
	maps.Copy(props, entity.Properties)

	cloneResult, err := h.manager.CreateEntity(r.Context(),
		&entityPkg.Entity{
			Type:       typeName,
			Properties: props,
			Content:    entity.Content,
		},
		entityPkg.CreateOptions{},
	)
	if err != nil {
		if writeForbiddenIfACLDenied(w, err) {
			return
		}
		writeV1Error(w, r, http.StatusInternalServerError, "clone_failed", "Failed to clone entity", err.Error())
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

	r = h.enterWrite(r)
	defer h.writeMu.Unlock()

	st := h.schema()

	cf, err := conflict.ParseConflictedFile(absPath, st.Meta)
	if err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "parse_failed", "Failed to parse conflict", err.Error())
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
		writeV1Error(w, r, http.StatusInternalServerError, "resolve_failed", "Failed to resolve", err.Error())
		return
	}
	if !h.authorizeConflictResolve(r.Context(), w, resolvedEntity, resolvedRelation) {
		return
	}
	if err := conflict.ValidateResolved(resolvedEntity, st.Meta); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "resolve_failed", "Failed to resolve", err.Error())
		return
	}
	if err := conflict.WriteResolved(absPath, resolvedEntity, resolvedRelation); err != nil {
		writeV1Error(w, r, http.StatusInternalServerError, "resolve_failed", "Failed to resolve", err.Error())
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
		var fromType string
		if fromEntity, ok := h.reader.getEntity(ctx, rel.From); ok {
			fromType = fromEntity.Type
		}
		aclReq = translateRelationWrite(rel.Type, fromType, rel.From)
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
