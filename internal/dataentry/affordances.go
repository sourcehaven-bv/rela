package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// translateVerb maps a wire-format verb to the [acl.WriteRequest] that
// authorizes that operation. It is the single source of truth for the
// "same code path" invariant: both the affordance serializer and the
// write handlers route their [acl.WriteRequest] construction through
// here. A grep test (`lint_test.go`) enforces that no other site in
// internal/dataentry constructs `acl.WriteRequest{Op:` directly.
//
// The verb set is closed and lives next to its callers; the only sites
// that pass verbs in are [perItemVerbs] and [perCollectionVerbs] in
// this file. Adding a new verb requires an entry here plus an
// [acl.Op] constant.
//
// entityID is the entity being acted on; empty for per-collection
// verbs (e.g. "create" against a type, no instance yet). It populates
// [acl.EntitySubject.ID] so the v1 ACL can evaluate entity-aware
// local-role grants (e.g. "alice can edit TKT-042 because she's
// assigned to it").
func translateVerb(verb, entityType, entityID string) acl.WriteRequest {
	subject := acl.EntitySubject{Type: entityType, ID: entityID}
	switch verb {
	case "create":
		return acl.WriteRequest{Op: acl.OpCreate, Subject: subject}
	case "update":
		return acl.WriteRequest{Op: acl.OpUpdate, Subject: subject}
	case "delete":
		return acl.WriteRequest{Op: acl.OpDelete, Subject: subject}
	case "rename":
		return acl.WriteRequest{Op: acl.OpRename, Subject: subject}
	}
	// Unreachable for the closed verb set above. A panic here would
	// signal a bug in a future commit — better than silently returning
	// a zero WriteRequest that maps every verb to OpCreate.
	panic("dataentry.translateVerb: unknown verb: " + verb)
}

// translateRelationWrite maps a relation write to the [acl.WriteRequest]
// that authorizes it, mirroring how entitymanager gates relation
// updates: Op=update with a [acl.RelationSubject] evaluated against the
// source entity's type. It lives here so the lint_test
// single-construction-site invariant covers relation writes too; the
// only caller today is the conflict-resolve handler, whose write is
// file-level and cannot route through entitymanager.
func translateRelationWrite(relType, fromType, fromID string) acl.WriteRequest {
	return acl.WriteRequest{Op: acl.OpUpdate, Subject: acl.RelationSubject{
		Type:     relType,
		FromType: fromType,
		FromID:   fromID,
	}}
}

// affordanceService computes the read-time affordance maps (_actions,
// per-field/relation verdicts) and runs the write-time affordance validation
// that gates field and relation writes. Extracted from App (TKT-N26KLB M5.2):
// it is the shared write-authorization seam every data-entry write funnels
// through.
//
// It holds the ACL and the field-verdict resolver, plus a per-request
// metamodel accessor (meta) — the metamodel can change on reload, so it MUST
// be fetched per call, never captured. The two relation-graph reads it needs
// (getEntity, currentEdgesByPeer) are injected as callbacks rather than pulling
// the relation plumbing in.
//
// IMPORTANT — two invariants this type must preserve:
//   - It shares the SAME acl.ACL instance as the write path
//     (entitymanager). affordances_contract_test.go pins the
//     "_actions[v]==false ⇒ 403 on the write" contract against that shared
//     instance; a divergent ACL here silently breaks it.
//   - acl.WriteRequest is constructed ONLY via the package-level translateVerb
//     / translateRelationWrite in this file (affordances.go); lint_test.go
//     greps this exact filename. Do not methodize those constructors or move
//     them to another file.
type affordanceService struct {
	// acl and resolver are per-call accessors (not captured values): both can be
	// swapped on App after construction (tests do `app.acl = …` /
	// `app.fieldResolver = …`), so a captured copy would go stale. Reading acl
	// live from App is also what structurally guarantees the affordance service
	// uses the SAME acl as the write path — the contract-test invariant.
	acl      func() acl.ACL
	resolver func() FieldVerdictResolver
	store    store.Store
	meta     func() *metamodel.Metamodel
	// getEntity resolves an entity by ID for relation-source attribution.
	getEntity func(ctx context.Context, id string) (*entityPkg.Entity, bool)
	// currentEdgesByPeer returns the current edges of entityID for a relation
	// type/direction, keyed by peer ID. Used to diff desired-vs-current edges.
	currentEdgesByPeer func(ctx context.Context, entityID, canonical string, incoming bool) map[string]*entityPkg.Relation
}

// perItemVerbs are the verbs computed per entity instance.
var perItemVerbs = []string{"update", "delete", "rename"}

// perCollectionVerbs are the verbs computed for the collection root.
var perCollectionVerbs = []string{"create"}

// computeActions returns the per-item verb verdict map for entity e.
// Every authenticated data-entry request reaches this through the
// router middleware, so the map is always populated for HTTP traffic;
// callers that synthesize their own context (tests, future non-HTTP
// callers) get the same `{verb: bool}` shape evaluated against
// whatever Principal is on ctx, defaulting to `principal.From`'s
// "unknown" sentinel.
func (svc affordanceService) computeActions(ctx context.Context, e *entityPkg.Entity) map[string]bool {
	out := make(map[string]bool, len(perItemVerbs))
	for _, v := range perItemVerbs {
		out[v] = svc.acl().AuthorizeWrite(ctx, translateVerb(v, e.Type, e.ID)).Allow
	}
	return out
}

// computeCollectionActions returns the collection-scope verb verdict
// map for an entity type — currently just `create`.
func (svc affordanceService) computeCollectionActions(ctx context.Context, entityType string) map[string]bool {
	out := make(map[string]bool, len(perCollectionVerbs))
	for _, v := range perCollectionVerbs {
		out[v] = svc.acl().AuthorizeWrite(ctx, translateVerb(v, entityType, "")).Allow
	}
	return out
}

// FieldVerdictResolver decides per-entity affordances for fields, enum
// options, and relation-meta fields. The wire shape it feeds into is
// documented in docs/data-entry/api-reference.md.
//
// v1 ships two implementations:
//
//   - [NopFieldVerdictResolver] — returns zero verdicts; every field,
//     option, and relation is permitted. Default unless
//     RELA_AFFORDANCE_PROFILE selects another.
//   - [DemoFieldVerdictResolver] — a hardcoded fixture against the
//     ticket type, exercising every affordance code path so the SPA
//     work in TKT-G7N5 has an observable end-to-end behavior to test
//     against.
//
// The eventual predicate-engine ticket replaces both with a
// policy-driven implementation that reads acl.yaml. The interface
// shape is intentionally narrow so the swap is mechanical.
type FieldVerdictResolver interface {
	FieldVerdicts(ctx context.Context, e *entityPkg.Entity) FieldVerdicts
	RelationVerdicts(ctx context.Context, e *entityPkg.Entity) RelationVerdicts
}

// FieldVerdicts carries per-entity field-level affordance decisions.
// All maps use sparse semantics: absence of a key means "default" (the
// permissive default — writable, visible, all options allowed). Only
// deviations need to be populated.
type FieldVerdicts struct {
	// Writable maps fieldName → writable. Absence = writable.
	Writable map[string]bool

	// Visible maps fieldName → visible. Absence = visible. False means
	// the property is omitted from the wire `properties` map AND from
	// `_fields`; the SPA's filter never sees the key.
	Visible map[string]bool

	// Options maps fieldName → optionValue → allowed. Absence of the
	// field OR absence of an option means allowed. Used for enum-typed
	// properties.
	Options map[string]map[string]bool

	// Attribution maps a denied path (field name, or "field=option")
	// to the role/grant that produced the deny. Audit-only — never
	// serialized to the wire. Sparse: only denials appear. Empty for
	// resolvers (Nop / Demo) that don't track attribution.
	Attribution map[string]string
}

// RelationVerdicts carries per-entity relation-level affordance
// decisions. The map is sparse: relation types not listed default to
// fully-permitted ({creatable: true, removable: true} with no
// meta-field restrictions).
type RelationVerdicts struct {
	Types map[string]RelationVerdict
}

// RelationVerdict carries the affordance decision for a single
// relation type. Zero-value (Creatable=false, Removable=false, Fields=nil)
// would deny everything; callers always populate explicitly.
type RelationVerdict struct {
	Creatable bool
	Removable bool
	// Fields maps metaField → writable. Absence = writable. Applies
	// uniformly to every link of this relation type (per-link
	// affordances are predicate territory, deferred).
	Fields map[string]bool

	// Attribution maps a denied dimension ("create", "remove",
	// "fields.<name>") to the role/grant that denied it. Audit-only,
	// never serialized. Sparse.
	Attribution map[string]string
}

// AffordanceDenialRule is the stable identifier surfaced in 403
// responses when an affordance validator rejects a write. The full
// rule_id on the wire is "<rule>:<path>" so a UI or audit reader can
// reconstruct what was denied.
//
// Rule names are part of the wire contract — changing them is a wire
// break.
type AffordanceDenialRule string

const (
	RuleFieldHidden          AffordanceDenialRule = "field-affordance:hidden"
	RuleFieldReadOnly        AffordanceDenialRule = "field-affordance:read-only"
	RuleFieldEnumFiltered    AffordanceDenialRule = "field-affordance:enum-filtered"
	RuleRelationNotCreatable AffordanceDenialRule = "relation-affordance:not-creatable"
	RuleRelationNotRemovable AffordanceDenialRule = "relation-affordance:not-removable"
	RuleRelationMetaReadOnly AffordanceDenialRule = "relation-affordance:meta-read-only"
)

// AffordanceDenialError reports why a write was rejected by the
// affordance validator. The rule and path together form the wire
// rule_id (e.g. "field-affordance:hidden:priority"). Reason is a
// short human-readable explanation; UIs surface it as-is.
type AffordanceDenialError struct {
	Rule   AffordanceDenialRule
	Path   string // property name, relation type, or "<relation-type>.<meta-field>"
	Reason string
	// Attribution names the role/grant that produced the deny, for the
	// audit Summary channel (DR-C5). Empty for resolvers that don't
	// track it. Never serialized to the wire 403 body.
	Attribution string
}

// RuleID returns the wire-stable identifier for this denial.
func (d AffordanceDenialError) RuleID() string {
	if d.Path == "" {
		return string(d.Rule)
	}
	return string(d.Rule) + ":" + d.Path
}

// Error makes AffordanceDenialError satisfy the error interface so it can
// flow back through caller chains. The format mirrors RuleID() plus
// the reason.
func (d AffordanceDenialError) Error() string {
	return d.RuleID() + ": " + d.Reason
}

// validateFieldWrite reports the first AffordanceDenialError that the
// proposed property writes trigger. Returns nil when every requested
// field is permitted.
//
// The validator handles four classes of denial:
//
//  1. Unknown fields (not declared in the metamodel) — rejected with
//     RuleFieldHidden so the response is byte-equivalent to a true
//     hidden-field rejection. This closes the F8 side channel.
//  2. Hidden fields — Visible[name] == false in the resolver verdict.
//  3. Read-only fields — Writable[name] == false. Strict: same-value
//     writes are not exempted (useAutoSave does no-op suppression
//     client-side; the server doesn't repeat that logic).
//  4. Filtered enum options — Options[name][value] == false.
//
// `setKeys` is the set of property names being written (from
// `properties` in the PATCH body); `unsetKeys` is the set being
// removed (`properties_unset`). Both are checked against the same
// rules: hidden/read-only fields cannot be set OR unset.
//
// Values are required only for the enum-filter check; pass the
// requested value for each key in setValues. Unknown values default
// to allowed (an option entry of `nil` means "no override").
func (svc affordanceService) validateFieldWrite(ctx context.Context, e *entityPkg.Entity, setKeys map[string]interface{}, unsetKeys []string) *AffordanceDenialError {
	if e == nil {
		return nil
	}
	v := svc.resolver().FieldVerdicts(ctx, e)
	declared := declaredProperties(svc.meta(), e.Type)

	check := func(key string, value interface{}, present bool) *AffordanceDenialError {
		// Unknown field (not in metamodel, not in resolver overrides) →
		// hidden-shape rejection (F8 side-channel closure).
		if !declared[key] && !knownToResolver(v, key) {
			return &AffordanceDenialError{
				Rule:   RuleFieldHidden,
				Path:   key,
				Reason: fmt.Sprintf("field %q is not visible", key),
			}
		}
		// Hidden via resolver verdict.
		if !v.IsVisible(key) {
			return &AffordanceDenialError{
				Rule:        RuleFieldHidden,
				Path:        key,
				Reason:      fmt.Sprintf("field %q is not visible", key),
				Attribution: v.Attribution[key],
			}
		}
		// Read-only via resolver verdict.
		if !v.IsWritable(key) {
			return &AffordanceDenialError{
				Rule:        RuleFieldReadOnly,
				Path:        key,
				Reason:      fmt.Sprintf("field %q is not writable", key),
				Attribution: v.Attribution[key],
			}
		}
		// Enum-filter (only for set, not unset, and only when a value
		// is provided — unset has no value to check). Handles both
		// scalar enums and list-typed enums (e.g. tags); for the list
		// case every element is checked against the allow-set and the
		// first disallowed value triggers the denial.
		if present && value != nil {
			if opts, ok := v.Options[key]; ok {
				if d := checkEnumOption(key, value, opts); d != nil {
					// checkEnumOption sets Path to "field=option", the
					// same key the resolver attributes options under.
					d.Attribution = v.Attribution[d.Path]
					return d
				}
			}
		}
		return nil
	}

	// Check sets first, then unsets. First denial wins.
	for k, val := range setKeys {
		if d := check(k, val, true); d != nil {
			return d
		}
	}
	for _, k := range unsetKeys {
		if d := check(k, nil, false); d != nil {
			return d
		}
	}
	return nil
}

// checkEnumOption rejects an enum value that isn't in the allow-set.
// Handles both scalar enums (`string`) and list-typed enums
// (`[]interface{}` — the JSON decoder's shape for a YAML
// `list: true` enum like `tags`). For lists, the first disallowed
// element produces the denial. Returns nil when the value passes.
//
// Non-string/non-list values fall through silently — the existing
// type-validation pipeline catches those upstream; the affordance
// gate only cares about disallowed-but-otherwise-valid values.
func checkEnumOption(key string, value interface{}, opts map[string]bool) *AffordanceDenialError {
	deny := func(option string) *AffordanceDenialError {
		return &AffordanceDenialError{
			Rule:   RuleFieldEnumFiltered,
			Path:   key + "=" + option,
			Reason: fmt.Sprintf("option %q is not allowed for field %q", option, key),
		}
	}
	switch v := value.(type) {
	case string:
		if allowed, ok := opts[v]; ok && !allowed {
			return deny(v)
		}
	case []interface{}:
		for _, elem := range v {
			str, ok := elem.(string)
			if !ok {
				continue
			}
			if allowed, ok := opts[str]; ok && !allowed {
				return deny(str)
			}
		}
	}
	return nil
}

// declaredProperties returns the set of property names that the
// metamodel declares for entityType. Returns an empty (non-nil) map
// when the entity type is unknown — callers should treat that as
// "nothing is declared," which causes the unknown-field rule to
// reject every PATCH key. That's deliberate: an unknown entity type
// should never reach this code path (the GET handler returns 404
// upstream), but if it does the safe-fail behavior is reject-all.
func declaredProperties(meta *metamodel.Metamodel, entityType string) map[string]bool {
	out := make(map[string]bool)
	if meta == nil {
		return out
	}
	def, ok := meta.Entities[entityType]
	if !ok {
		return out
	}
	for name := range def.Properties {
		out[name] = true
	}
	return out
}

// knownToResolver reports whether the resolver has any verdict
// (writable, visible, or options) covering the given field name.
// Used as the "known field" fallback for fields the metamodel does
// not declare but the resolver has explicit opinions on.
func knownToResolver(v FieldVerdicts, name string) bool {
	if _, ok := v.Writable[name]; ok {
		return true
	}
	if _, ok := v.Visible[name]; ok {
		return true
	}
	if _, ok := v.Options[name]; ok {
		return true
	}
	return false
}

// writeAffordanceDenialError renders an AffordanceDenialError as a 403
// response. The wire shape mirrors writeForbiddenIfACLDenied (the
// ACL helper) so SPA error-handling can treat the two uniformly.
//
// Prefer [App.denyAffordance] when handler context is available — it
// emits the audit row in addition to writing the response.
func writeAffordanceDenialError(w http.ResponseWriter, denial AffordanceDenialError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":     "forbidden",
		"rule_kind": "affordance",
		"rule_id":   denial.RuleID(),
		"reason":    denial.Reason,
	})
}

// denyAffordance writes the 403 response AND records a `denied-write`
// audit row attributed to the request principal. Use from every
// affordance-gate site so the audit stream is uniform with ACL
// denials (which the entitymanager emits the same op for).
//
// `target` is the entity the gate fired on — used to populate the
// audit Subject so log readers can attribute the denial. Nil is
// tolerated (subject left empty); callers always have it in practice.
func (a *App) denyAffordance(ctx context.Context, w http.ResponseWriter, target *entityPkg.Entity, denial AffordanceDenialError) {
	var subject *audit.Subject
	if target != nil {
		subject = &audit.Subject{
			Kind: "entity",
			Type: target.Type,
			ID:   target.ID,
		}
	}
	summary := fmt.Sprintf("denied: %s (rule_kind=affordance rule_id=%s)",
		denial.Reason, denial.RuleID())
	if denial.Attribution != "" {
		summary += " attribution=" + denial.Attribution
	}
	a.auditSink.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpDeniedWrite,
		Subject:     subject,
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     summary,
	})
	writeAffordanceDenialError(w, denial)
}

// RelationOp identifies which relation-write operation a caller is
// gating. Pass via [App.validateRelationOp].
type RelationOp int

const (
	// RelationOpCreate gates adding an edge of the given type.
	RelationOpCreate RelationOp = iota
	// RelationOpRemove gates removing any edge of the given type.
	RelationOpRemove
)

// relationSourceEntity returns the entity whose verdict should gate a
// per-relation write. For outgoing-direction operations the source IS
// the path entity (the canonical case). For incoming-direction
// operations the path entity is the TARGET; the source is the peer,
// so the resolver must be asked about the peer's affordance, not the
// path entity's.
//
// Returns the path entity when the peer can't be found locally (404
// upstream — should not happen in practice; safe-fail).
func (svc affordanceService) relationSourceEntity(
	ctx context.Context, pathEntity *entityPkg.Entity, peerID, direction string,
) *entityPkg.Entity {
	if direction != string(DirectionIncoming) {
		return pathEntity
	}
	peer, ok := svc.getEntity(ctx, peerID)
	if !ok {
		return pathEntity
	}
	return peer
}

// validateRelationOp reports the first AffordanceDenialError that the
// proposed relation operation triggers. Returns nil when permitted.
// `relType` is the canonical relation type; `op` selects between
// create / remove. The verdict is per-relation-type uniform —
// per-link affordances are predicate territory.
func (svc affordanceService) validateRelationOp(ctx context.Context, e *entityPkg.Entity, relType string, op RelationOp) *AffordanceDenialError {
	if e == nil {
		return nil
	}
	v := svc.resolver().RelationVerdicts(ctx, e)
	rv, ok := v.Types[relType]
	if !ok {
		return nil // default-permissive
	}
	switch op {
	case RelationOpCreate:
		if !rv.Creatable {
			return &AffordanceDenialError{
				Rule:        RuleRelationNotCreatable,
				Path:        relType,
				Reason:      fmt.Sprintf("relation %q is not creatable", relType),
				Attribution: rv.Attribution["create"],
			}
		}
	case RelationOpRemove:
		if !rv.Removable {
			return &AffordanceDenialError{
				Rule:        RuleRelationNotRemovable,
				Path:        relType,
				Reason:      fmt.Sprintf("relation %q is not removable", relType),
				Attribution: rv.Attribution["remove"],
			}
		}
	}
	return nil
}

// validateRelationsModernAffordances reports the first
// AffordanceDenialError that any of the proposed relation diffs trigger
// across the unified-PATCH modern body. Diffs against current edges
// to identify true adds and removes (rather than upserts), and
// inspects per-edge meta against [RelationVerdict.Fields].
//
// Called from the unified PATCH handler before
// [App.applyRelationsModern]. Returns nil when every relation
// operation is permitted.
func (svc affordanceService) validateRelationsModernAffordances(
	ctx context.Context, entityID string, e *entityPkg.Entity,
	desired map[string]V1RelationsUpdate,
) *AffordanceDenialError {
	if e == nil || len(desired) == 0 {
		return nil
	}
	meta := svc.meta()
	for bodyKey, upd := range desired {
		if !upd.DataPresent {
			continue
		}
		canonical, incoming, ok := resolveDirection(meta, bodyKey)
		if !ok {
			continue // structural error surfaces via the existing validator
		}

		desiredByID := make(map[string]V1ResourceIdentifier, len(upd.Data))
		for _, ref := range upd.Data {
			desiredByID[ref.ID] = ref
		}
		current := svc.currentEdgesByPeer(ctx, entityID, canonical, incoming)

		// For incoming-direction body keys the SOURCE of every edge is
		// the peer entity, not the path entity. Verdicts are evaluated
		// against the source — see [App.relationSourceEntity] for the
		// rationale. Outgoing edges resolve to the path entity.
		direction := ""
		if incoming {
			direction = string(DirectionIncoming)
		}

		// Adds: any desired edge whose peer isn't currently linked.
		for _, ref := range upd.Data {
			source := svc.relationSourceEntity(ctx, e, ref.ID, direction)
			if _, exists := current[ref.ID]; exists {
				// Upsert path: not a create, but the meta may change.
				if denial := svc.validateRelationMetaWrite(ctx, source, canonical, ref.Meta, ref.MetaUnset); denial != nil {
					return denial
				}
				continue
			}
			if denial := svc.validateRelationOp(ctx, source, canonical, RelationOpCreate); denial != nil {
				return denial
			}
			if denial := svc.validateRelationMetaWrite(ctx, source, canonical, ref.Meta, ref.MetaUnset); denial != nil {
				return denial
			}
		}

		// Removes: any current edge not in the desired set.
		for peerID := range current {
			if _, kept := desiredByID[peerID]; kept {
				continue
			}
			source := svc.relationSourceEntity(ctx, e, peerID, direction)
			if denial := svc.validateRelationOp(ctx, source, canonical, RelationOpRemove); denial != nil {
				return denial
			}
		}
	}
	return nil
}

// validateRelationMetaWrite reports the first AffordanceDenialError that
// the proposed relation-meta writes trigger. Set+unset are both
// treated as writes (F16). Returns nil when permitted.
//
// `meta` is the proposed property map; `metaUnset` lists keys being
// removed. Unknown meta keys (not in the resolver's Fields map AND
// not declared in the metamodel for this relation type) are not
// rejected here — they're a separate concern handled by the existing
// relation validation. The affordance check focuses on rejecting
// keys explicitly marked non-writable.
func (svc affordanceService) validateRelationMetaWrite(ctx context.Context, e *entityPkg.Entity, relType string, meta map[string]interface{}, metaUnset []string) *AffordanceDenialError {
	if e == nil {
		return nil
	}
	v := svc.resolver().RelationVerdicts(ctx, e)
	rv, ok := v.Types[relType]
	if !ok || rv.Fields == nil {
		return nil
	}
	deny := func(key string) *AffordanceDenialError {
		if writable, ok := rv.Fields[key]; ok && !writable {
			return &AffordanceDenialError{
				Rule:        RuleRelationMetaReadOnly,
				Path:        relType + "." + key,
				Reason:      fmt.Sprintf("meta field %q on relation %q is not writable", key, relType),
				Attribution: rv.Attribution["fields."+key],
			}
		}
		return nil
	}
	for k := range meta {
		if d := deny(k); d != nil {
			return d
		}
	}
	for _, k := range metaUnset {
		if d := deny(k); d != nil {
			return d
		}
	}
	return nil
}

// computeFieldAffordances returns the sparse `_fields` wire map for
// entity e: only fields whose verdict deviates from the permissive
// default appear. Hidden fields (Visible[name] == false) are absent
// from the returned map AND must be omitted from the entity's
// Properties by the caller — they are doubly-invisible to the client.
//
// Empty input verdicts yield an empty (non-nil) map so the wire shape
// is consistent: `_fields: {}` under the nop resolver, sparse entries
// under any other.
func (svc affordanceService) computeFieldAffordances(ctx context.Context, e *entityPkg.Entity) map[string]V1FieldAffordance {
	return computeFieldAffordancesFrom(svc.resolver().FieldVerdicts(ctx, e))
}

// computeFieldAffordancesFrom is computeFieldAffordances given an
// already-resolved verdict set, so callers that need the verdicts for more
// than one thing (e.g. _fields + _attachments) resolve them once.
func computeFieldAffordancesFrom(v FieldVerdicts) map[string]V1FieldAffordance {
	out := make(map[string]V1FieldAffordance)

	// writable=false entries
	for name, writable := range v.Writable {
		if writable {
			continue // sparse: default is writable
		}
		if !v.Visible[name] && v.isHidden(name) {
			continue // hidden takes precedence; skip from _fields entirely
		}
		entry := out[name]
		f := false
		entry.Writable = &f
		out[name] = entry
	}

	// option-filter entries
	for name, opts := range v.Options {
		if v.isHidden(name) {
			continue
		}
		var falseOpts map[string]bool
		for opt, allowed := range opts {
			if allowed {
				continue
			}
			if falseOpts == nil {
				falseOpts = make(map[string]bool)
			}
			falseOpts[opt] = false
		}
		if falseOpts == nil {
			continue
		}
		entry := out[name]
		entry.Options = falseOpts
		out[name] = entry
	}

	return out
}

// IsWritable reports whether name is writable. The default is true —
// absent or true-valued entries both yield true; only explicit false
// values are denials.
func (v FieldVerdicts) IsWritable(name string) bool {
	writable, ok := v.Writable[name]
	return !ok || writable
}

// IsVisible reports whether name is visible. The default is true —
// absent or true-valued entries both yield true; only explicit false
// values hide the field.
func (v FieldVerdicts) IsVisible(name string) bool {
	visible, ok := v.Visible[name]
	return !ok || visible
}

// IsOptionAllowed reports whether option `opt` is allowed for the
// enum-typed field `name`. The default is allowed — absent or
// true-valued entries both yield true; only explicit false values
// filter the option out.
func (v FieldVerdicts) IsOptionAllowed(name, opt string) bool {
	opts, ok := v.Options[name]
	if !ok {
		return true
	}
	allowed, ok := opts[opt]
	return !ok || allowed
}

// isHidden reports whether v marks name as hidden. Returns false for
// the absent-key case (default is visible). Internal sibling to
// [FieldVerdicts.IsVisible] — kept for the existing callers that
// phrase the check in the negative form.
func (v FieldVerdicts) isHidden(name string) bool { return !v.IsVisible(name) }

// hiddenProperties returns the set of property names that should be
// stripped from V1Entity.Properties before serialization. Caller uses
// this to enforce the omit-on-hidden invariant.
func (svc affordanceService) hiddenProperties(ctx context.Context, e *entityPkg.Entity) map[string]struct{} {
	v := svc.resolver().FieldVerdicts(ctx, e)
	if len(v.Visible) == 0 {
		return nil
	}
	out := make(map[string]struct{})
	for name, visible := range v.Visible {
		if !visible {
			out[name] = struct{}{}
		}
	}
	return out
}

// computeRelationAffordances returns the sparse `_relations` wire map
// for entity e. Only relation types with at least one deviation
// (creatable=false, removable=false, or any meta-field writable=false)
// appear in the map. Default-permissive types are absent — the SPA's
// "no entry = default" path handles them.
func (svc affordanceService) computeRelationAffordances(ctx context.Context, e *entityPkg.Entity) map[string]V1RelationAffordance {
	v := svc.resolver().RelationVerdicts(ctx, e)
	out := make(map[string]V1RelationAffordance)
	for relType, rv := range v.Types {
		var entry V1RelationAffordance
		emit := false
		if !rv.Creatable {
			f := false
			entry.Creatable = &f
			emit = true
		}
		if !rv.Removable {
			f := false
			entry.Removable = &f
			emit = true
		}
		var fields map[string]V1FieldAffordance
		for metaField, writable := range rv.Fields {
			if writable {
				continue
			}
			if fields == nil {
				fields = make(map[string]V1FieldAffordance)
			}
			f := false
			fields[metaField] = V1FieldAffordance{Writable: &f}
		}
		if fields != nil {
			entry.Fields = fields
			emit = true
		}
		if emit {
			out[relType] = entry
		}
	}
	return out
}

// copyVisibleProperties returns a fresh map of the entity's properties
// with hidden names filtered out, ready to ship on a per-row wire
// surface (cards/list rows in V1ViewEntity._props). Shallow copy: each
// value points at the same underlying object as e.Properties[k], which
// is fine because the response is JSON-marshaled before the caller can
// alias anything (TKT-IHC7D).
//
// Mirrors stripHiddenProperties's hidden-property contract but returns
// a new map instead of mutating an existing V1Entity — the per-row
// case never goes through V1Entity, so the in-place strip pattern
// doesn't fit.
func (svc affordanceService) copyVisibleProperties(ctx context.Context, e *entityPkg.Entity) map[string]any {
	hidden := svc.hiddenProperties(ctx, e)
	out := make(map[string]any, len(e.Properties))
	for k, v := range e.Properties {
		if _, h := hidden[k]; h {
			continue
		}
		out[k] = v
	}
	return out
}

// stripHiddenProperties removes hidden field names from result.Properties
// in-place. Centralizes the "hidden = omitted from wire" invariant so
// every entity-returning response (GET, PATCH, POST, clone, action,
// includes) honors it consistently.
//
// Also rewrites `_title` to the entity ID when the entity-type's
// display property is hidden — otherwise the display title would leak
// the hidden value through the wire's secondary channel.
func (svc affordanceService) stripHiddenProperties(ctx context.Context, e *entityPkg.Entity, result *V1Entity) {
	hidden := svc.hiddenProperties(ctx, e)
	for name := range hidden {
		delete(result.Properties, name)
	}
	if len(hidden) == 0 {
		return
	}
	def, ok := svc.meta().Entities[e.Type]
	if !ok {
		return
	}
	primary := def.GetPrimaryProperty()
	if primary == "" {
		return
	}
	if _, hiddenPrimary := hidden[primary]; hiddenPrimary {
		// Fall back to the entity ID, matching DisplayTitle's
		// missing-property branch. The ID is non-secret by design.
		result.Title = e.ID
	}
}

// attachEntityAffordances writes the per-entity `_fields` and
// `_relations` wire maps onto result. Called by paths that return a
// per-entity response (GET, PATCH, POST, clone, action) — list rows
// and includes get [App.stripHiddenProperties] only.
func (svc affordanceService) attachEntityAffordances(ctx context.Context, e *entityPkg.Entity, result *V1Entity) {
	verdicts := svc.resolver().FieldVerdicts(ctx, e)
	fields := computeFieldAffordancesFrom(verdicts)
	relations := svc.computeRelationAffordances(ctx, e)
	result.FieldAffordances = &fields
	result.RelationAffordances = &relations
	// Pass the same verdicts so a policy-hidden `file` property's attachments
	// are omitted from `_attachments` — otherwise the hidden-field boundary
	// the rest of the response maintains would leak the file's metadata and a
	// working download href.
	attachments := svc.computeAttachments(ctx, e, result.Self, verdicts)
	result.Attachments = &attachments
}

// computeAttachments returns the per-property attachment metadata for an
// entity, keyed by `file`-type property name. Only properties that carry
// a file appear; an empty map means "no attachments". Rides every
// per-entity V1Entity response (GET, PATCH, POST, clone) alongside
// `_fields` / `_relations`, never on list rows — same closed-world shape.
//
// selfHref is the entity's `_self` link (`/api/v1/{plural}/{id}`); each
// file's download href is that plus `/_attachments/{property}/{fileName}`.
// `_self` is always set by entityToV1 before this runs, so the
// empty-selfHref guard is pure defense — when it can't build a valid href
// it omits the entry rather than emit a broken relative link.
//
// The value per property is a LIST: a property may hold several files, and
// even a single file is reported as a 1-element list (always-array wire
// shape).
func (svc affordanceService) computeAttachments(ctx context.Context, e *entityPkg.Entity, selfHref string, verdicts FieldVerdicts) map[string][]V1Attachment {
	out := make(map[string][]V1Attachment)
	if selfHref == "" {
		return out
	}
	infos, err := svc.store.ListAttachments(ctx, e.ID)
	if err != nil {
		// Treat a list failure as "no attachments" rather than failing the
		// whole entity response — the bytes endpoint still gates and serves
		// correctly; this map is only a UI hint. But a real backend fault
		// (not just a missing entity) is logged so operators aren't blind to
		// an outage that silently empties every entity's _attachments.
		if !errors.Is(err, store.ErrNotFound) {
			slog.Warn("dataentry: list attachments for serialization failed",
				"err", err, "entity", e.ID)
		}
		return out
	}
	for _, info := range infos {
		// A property hidden from this viewer by field-visibility policy must
		// not leak its files (metadata or a working download href) — mirror
		// the hidden-field boundary the rest of the response maintains.
		if !verdicts.IsVisible(info.Property) {
			continue
		}
		out[info.Property] = append(out[info.Property], V1Attachment{
			ID:          info.FileName,
			FileName:    info.FileName,
			Size:        info.Size,
			ContentType: contentTypeForFilename(info.FileName),
			Href:        selfHref + "/_attachments/" + info.Property + "/" + url.PathEscape(info.FileName),
		})
	}
	return out
}

// serializeEntityForWire is the single entry-point every handler that
// returns a per-entity V1Entity should use. It calls entityToV1, strips
// hidden properties, and attaches the affordance maps. Use
// [App.serializeRelatedEntityForWire] for entities that appear as
// list rows or under `included` (no affordance maps, but still strip).
func (a *App) serializeEntityForWire(ctx context.Context, e *entityPkg.Entity, plural string, includeRelations bool) V1Entity {
	result := a.entityToV1(ctx, e, plural, includeRelations)
	a.affordances.stripHiddenProperties(ctx, e, &result)
	a.affordances.attachEntityAffordances(ctx, e, &result)
	return result
}

// serializeRelatedEntityForWire renders an entity that is NOT the
// per-entity response root — used for list rows, `?include=*` peers,
// and the search-result include map. Strips hidden properties but
// omits the `_fields` / `_relations` maps (they ride on per-entity
// responses only). Hidden-field stripping still applies because the
// wire contract is "hidden values never reach the client, regardless
// of which response shape they ride in."
func (a *App) serializeRelatedEntityForWire(ctx context.Context, e *entityPkg.Entity, plural string, includeRelations bool) V1Entity {
	result := a.entityToV1(ctx, e, plural, includeRelations)
	a.affordances.stripHiddenProperties(ctx, e, &result)
	return result
}
