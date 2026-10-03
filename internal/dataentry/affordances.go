package dataentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"sort"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/attachment"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
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
//
// face names the CONTENT STATE being acted on, and is REQUIRED rather
// than optional: an affordance must answer the same question the write
// will (BUG-Y0GNSB). Passing the zero face where a real one applies
// reports `update: true` on a face the principal cannot write, which is
// how the escalation this fixes stayed invisible in the API response.
// Pass the face off the entity being described; the zero value is
// correct only for a genuinely unfaced entity or a per-collection verb.
func translateVerb(verb, entityType, entityID string, face entityPkg.Face) acl.WriteRequest {
	subject := acl.NewEntitySubject(entityType, entityID, face)
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

// translateRelationDelete maps the removal of a relType edge whose source is
// fromID at face to the [acl.WriteRequest] the manager authorizes it with.
func translateRelationDelete(relType, fromType, fromID string, face entityPkg.Face) acl.WriteRequest {
	return acl.WriteRequest{Op: acl.OpDelete, Subject: acl.RelationSubject{
		Type: relType, FromType: fromType, FromID: fromID, FromFace: face,
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
// (getEntity, planEdges) are injected as callbacks rather than pulling
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
	// family reads the faces of one entity the principal may read
	// ([visibility.Resolver.Family]). `_faces` is built from it.
	family func(ctx context.Context, entityType, id string) (visibility.Family, bool, error)
	// sourceRow reads the raw row of a relation's source, at the edge's
	// tail, for relation-source attribution. It is never served.
	sourceRow func(ctx context.Context, ref entityPkg.Ref) (*entityPkg.Entity, bool)
	// sourceFamily reads the raw row of every face of a relation's source,
	// for a write gate whose source has no row at the edge's tail. It is
	// never served either.
	sourceFamily func(ctx context.Context, id string) ([]*entityPkg.Entity, error)
	// copies lists the copy affordances available from one face (RULING 9's
	// promote / translate buttons). OPTIONAL, unlike the accessors above: nil
	// when the entity manager does not expose the capability, in which case
	// `_copies` is omitted rather than sent empty — see computeCopyOffers.
	// Wired by wireCopies, in production and in the test rebind alike.
	copies copyOffersFunc
	// planEdges is edgeReader.plan: the edge writes one relation wrapper
	// asks for, which this service authorizes one by one.
	planEdges edgePlanner
	// schema and actionConditions back the detail-action affordance
	// (TKT-VVS16W). Live accessors because config reloads and the condition
	// compiler is injected after construction (SetViewConditions). Nil
	// schema offers no detail actions; nil actionConditions offers none
	// that declare a when.
	schema           func() *Schema
	actionConditions func() ViewConditionFunc
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
		if v == "rename" {
			out[v] = svc.renameAllowed(ctx, e)
			continue
		}
		out[v] = svc.acl().AuthorizeWrite(ctx, translateVerb(v, e.Type, e.ID, e.Face)).Allow
	}
	return out
}

// renameAllowed is the rename verdict for e. A rename moves the whole
// family, so the manager authorizes it on every stored face (BUG-GJUBSA),
// and so does this: the served face alone would offer a rename the write
// refuses. Delete stays per face, because a DELETE on `ID@face` removes
// that face only. A failed family read answers false.
func (svc affordanceService) renameAllowed(ctx context.Context, e *entityPkg.Entity) bool {
	if e.Face.IsImplicit() || svc.sourceFamily == nil {
		return svc.acl().AuthorizeWrite(ctx, translateVerb("rename", e.Type, e.ID, e.Face)).Allow
	}
	family, err := svc.sourceFamily(ctx, e.ID)
	if err != nil || len(family) == 0 {
		return false
	}
	for _, f := range family {
		if !svc.acl().AuthorizeWrite(ctx, translateVerb("rename", f.Type, f.ID, f.Face)).Allow {
			return false
		}
	}
	return true
}

// computeCollectionActions returns the collection-scope verb verdict
// map for an entity type, currently just `create`.
//
// A faced type has no implicit face to create on, so its `create` is true
// when any declared face is creatable, and each face gets its own
// `create@<face>` key. A create form uses those keys to offer the faces the
// principal may create when its world declares no `create:` face.
func (svc affordanceService) computeCollectionActions(ctx context.Context, entityType string) map[string]bool {
	out := make(map[string]bool, len(perCollectionVerbs))
	faces := metamodel.FaceOrderOf(svc.meta(), entityType)
	for _, v := range perCollectionVerbs {
		if len(faces) == 0 {
			out[v] = svc.acl().AuthorizeWrite(ctx, translateVerb(v, entityType, "", "")).Allow
			continue
		}
		allowed := false
		for _, f := range faces {
			ok := svc.acl().AuthorizeWrite(ctx, translateVerb(v, entityType, "", entityPkg.Face(f))).Allow
			out[v+"@"+f] = ok
			allowed = allowed || ok
		}
		out[v] = allowed
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

// traversalPrimer is the OPTIONAL capability of a [FieldVerdictResolver]
// whose grants evaluate `related(...)` (TKT-205V2N): it answers those for a
// whole page at once and returns a ctx carrying the answers. Only the
// policy-backed resolver has it.
type traversalPrimer interface {
	PrimeTraversals(ctx context.Context, rows []*entityPkg.Entity) context.Context
}

// primeVerdicts primes res for a page of rows so serializing the page costs
// one store query per grant traversal, not one per row. Serializing on the
// unprimed ctx is still correct, only slower.
func primeVerdicts(ctx context.Context, res FieldVerdictResolver, rows []*entityPkg.Entity) context.Context {
	if p, ok := res.(traversalPrimer); ok && len(rows) > 0 {
		return p.PrimeTraversals(ctx, rows)
	}
	return ctx
}

// TransitionResolver is the OPTIONAL sibling of [FieldVerdictResolver] that
// answers state-machine transition verdicts for an entity (TKT-3G93B8). It is
// kept separate — and type-asserted, not embedded — because only the
// policy-backed resolver can answer it; the Nop and Demo resolvers don't
// implement it, so `_transitions` is simply absent under them (the SPA falls
// back to the ordinary enum control). This mirrors the store's optional
// capabilities (e.g. HistoryReader), which are also type-asserted rather than
// forced into the base interface.
//
// The returned map is keyed by property name; each value is the resolved
// outgoing transitions for the ctx principal on e. An empty map means "no
// machine-typed property on this entity" (or no machines wired).
type TransitionResolver interface {
	TransitionVerdicts(ctx context.Context, e *entityPkg.Entity) map[string][]statemachine.TransitionVerdict
	// EntryValues returns, per state-machine-typed property of entityType, the
	// value a create must enter at (the machine's Initial/Default — BUG-X1C7S).
	// The create form uses it to lock the field to its initial value. Empty when
	// entityType has no machine-typed property.
	EntryValues(entityType string) map[string]string
}

// RelationVisibilityResolver is the OPTIONAL sibling of [FieldVerdictResolver]
// that answers per-meta-field READ visibility for one relation edge
// (TKT-B1F5Q1). Kept separate and type-asserted (not embedded) for the same
// reason as [TransitionResolver]: only the policy-backed resolver can express a
// relation `visible:` grant, so the Nop and Demo resolvers don't implement it
// and relation meta is emitted un-redacted under them (matching how relations
// behaved before B1F5Q1). This mirrors the store's optional capabilities.
//
// from is the source entity that owns the relation grant block; relType is the
// edge's relation type; metaKeys are the property names actually present on the
// edge about to be serialized (the closed-world deny universe). The result is
// sparse: an absent key means visible, a `false` value means hide that meta key.
type RelationVisibilityResolver interface {
	RelationFieldVerdicts(
		ctx context.Context, from *entityPkg.Entity, relType string, metaKeys []string,
	) map[string]bool
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

// fieldVerdicts overlays schema-intrinsic read-only fields onto the
// policy-derived verdict. Computed properties are materialized for display but
// can never be authored, regardless of ACL role.
func (svc affordanceService) fieldVerdicts(ctx context.Context, e *entityPkg.Entity) FieldVerdicts {
	v := svc.resolver().FieldVerdicts(ctx, e)
	if e == nil {
		return v
	}
	def, ok := svc.meta().GetEntityDef(e.Type)
	if !ok {
		return v
	}
	v.Writable = maps.Clone(v.Writable)
	if v.Writable == nil {
		v.Writable = map[string]bool{}
	}
	for name, pd := range def.Properties {
		if pd.Computed != "" {
			v.Writable[name] = false
		}
	}
	return v
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
func (svc affordanceService) validateFieldWrite(
	ctx context.Context, e *entityPkg.Entity, setKeys map[string]any, unsetKeys []string,
) *AffordanceDenialError {
	if e == nil {
		return nil
	}
	v := svc.fieldVerdicts(ctx, e)
	declared := declaredProperties(svc.meta(), e.Type)

	check := func(key string, value any, present bool) *AffordanceDenialError {
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
func checkEnumOption(key string, value any, opts map[string]bool) *AffordanceDenialError {
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
	case []any:
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
// Prefer App.denyAffordance when handler context is available — it
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
func (a *App) denyAffordance(
	ctx context.Context, w http.ResponseWriter, target *entityPkg.Entity, denial AffordanceDenialError,
) {
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
// gating. Pass via App.validateRelationOp.
type RelationOp int

const (
	// RelationOpCreate gates adding an edge of the given type.
	RelationOpCreate RelationOp = iota
	// RelationOpRemove gates removing any edge of the given type.
	RelationOpRemove
)

// relationSources returns the rows whose verdicts gate a per-relation write.
// For outgoing-direction operations the source IS the path entity (the
// canonical case). For incoming-direction operations the path entity is the
// TARGET; the source is the peer, so the resolver must be asked about the
// peer's affordance, not the path entity's.
//
// peer is the source at the edge's tail: the tail an existing edge carries, or
// the tail a new incoming edge will get. When the peer has no row there, the
// gate uses every row of its family, and a write passes only if every face
// permits it. That is the case for a new or identity-scoped edge from a faced
// peer: its zero face holds no row (DEC-NPZICR), and falling back to the path
// entity would judge the edge by the wrong type's policy.
//
// A peer with no stored row at all falls back to the path entity. No edge to
// it can be written, because the manager refuses a missing endpoint, and the
// fallback keeps the answer for a missing peer what it was.
//
// A read fault is returned: the caller must not write on a guess.
func (svc affordanceService) relationSources(
	ctx context.Context, pathEntity *entityPkg.Entity, peer entityPkg.Ref, direction string,
) ([]*entityPkg.Entity, error) {
	if direction != string(DirectionIncoming) {
		return []*entityPkg.Entity{pathEntity}, nil
	}
	if src, ok := svc.sourceRow(ctx, peer); ok {
		return []*entityPkg.Entity{src}, nil
	}
	family, err := svc.sourceFamily(ctx, peer.ID)
	if err != nil {
		return nil, fmt.Errorf("reading relation source %s: %w", peer.ID, err)
	}
	if len(family) == 0 {
		return []*entityPkg.Entity{pathEntity}, nil
	}
	return family, nil
}

// relationOpDenial is [affordanceService.validateRelationOp] over every row
// [affordanceService.relationSources] returned. It reports the first denial
// and the row that produced it, which is the write's audit subject.
func (svc affordanceService) relationOpDenial(
	ctx context.Context, sources []*entityPkg.Entity, relType string, op RelationOp,
) (*entityPkg.Entity, *AffordanceDenialError) {
	for _, src := range sources {
		if denial := svc.validateRelationOp(ctx, src, relType, op); denial != nil {
			return src, denial
		}
	}
	return nil, nil
}

// relationMetaDenial is [affordanceService.validateRelationMetaWrite] over
// every source row, like [affordanceService.relationOpDenial].
func (svc affordanceService) relationMetaDenial(
	ctx context.Context, sources []*entityPkg.Entity, relType string, meta map[string]any, metaUnset []string,
) (*entityPkg.Entity, *AffordanceDenialError) {
	for _, src := range sources {
		if denial := svc.validateRelationMetaWrite(ctx, src, relType, meta, metaUnset); denial != nil {
			return src, denial
		}
	}
	return nil, nil
}

// validateRelationOp reports the first AffordanceDenialError that the
// proposed relation operation triggers. Returns nil when permitted.
// `relType` is the canonical relation type; `op` selects between
// create / remove. The verdict is per-relation-type uniform —
// per-link affordances are predicate territory.
func (svc affordanceService) validateRelationOp(
	ctx context.Context, e *entityPkg.Entity, relType string, op RelationOp,
) *AffordanceDenialError {
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
// [writeHandler.applyRelationsModern]. Returns nil when every relation
// operation is permitted, a *AffordanceDenialError for a denial, and any other
// error when a source row cannot be read.
func (svc affordanceService) validateRelationsModernAffordances(
	ctx context.Context, entityID string, e *entityPkg.Entity,
	desired map[string]v1.RelationsUpdate,
) error {
	if e == nil || len(desired) == 0 {
		return nil
	}
	meta := svc.meta()
	for bodyKey, upd := range desired {
		if !upd.DataPresent && !upd.Delta {
			continue
		}
		canonical, incoming, ok := resolveDirection(meta, bodyKey)
		if !ok {
			continue // structural error surfaces via the existing validator
		}
		var ops []edgeOp
		if entityID == "" {
			// A create has no current edges to match: every listed edge is new.
			ops = newEdgeOps(upd, incoming)
		} else {
			var err error
			ops, err = svc.planEdges(ctx, entityID, e.Face, canonical, incoming, upd,
				"/relations/"+v1.JSONPointerEscape(bodyKey))
			if err != nil {
				return err
			}
		}
		if err := svc.validateRelationOps(ctx, e, canonical, incoming, ops); err != nil {
			return err
		}
	}
	return nil
}

// newEdgeOps is the plan for a wrapper on an entity being created.
func newEdgeOps(upd v1.RelationsUpdate, incoming bool) []edgeOp {
	refs := upd.Upserts()
	ops := make([]edgeOp, len(refs))
	for i, ref := range refs {
		peer := peerAddress(ref.ID, incoming)
		s := edgeSlot{peer: peer.ID()}
		if named, ok := peer.Named(); ok {
			s.tail = named.Face
		}
		ops[i] = edgeOp{slot: s, ref: ref}
	}
	return ops
}

// validateRelationOps authorizes the planned writes of one relation type.
// The source of an incoming edge is the peer at the edge's tail; see
// [affordanceService.relationSources].
func (svc affordanceService) validateRelationOps(
	ctx context.Context, e *entityPkg.Entity, canonical string, incoming bool, ops []edgeOp,
) error {
	direction := ""
	if incoming {
		direction = string(DirectionIncoming)
	}
	for _, op := range ops {
		sources, err := svc.relationSources(ctx, e, entityPkg.Ref{ID: op.slot.peer, Face: op.slot.tail}, direction)
		if err != nil {
			return err
		}
		switch {
		case op.remove:
			if _, denial := svc.relationOpDenial(ctx, sources, canonical, RelationOpRemove); denial != nil {
				return denial
			}
			continue
		case op.existing == nil:
			if _, denial := svc.relationOpDenial(ctx, sources, canonical, RelationOpCreate); denial != nil {
				return denial
			}
		}
		if _, denial := svc.relationMetaDenial(ctx, sources, canonical, op.ref.Meta, op.ref.MetaUnset); denial != nil {
			return denial
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
func (svc affordanceService) validateRelationMetaWrite(
	ctx context.Context, e *entityPkg.Entity, relType string, meta map[string]any, metaUnset []string,
) *AffordanceDenialError {
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
func (svc affordanceService) computeFieldAffordances(
	ctx context.Context, e *entityPkg.Entity,
) map[string]v1.FieldAffordance {
	return computeFieldAffordancesFrom(svc.fieldVerdicts(ctx, e))
}

// computeFieldAffordancesFrom is computeFieldAffordances given an
// already-resolved verdict set, so callers that need the verdicts for more
// than one thing (e.g. _fields + _attachments) resolve them once.
func computeFieldAffordancesFrom(v FieldVerdicts) map[string]v1.FieldAffordance {
	out := make(map[string]v1.FieldAffordance)

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

// hidesAnyField reports whether the active resolver can ever hide a property.
// The Nop resolver returns empty verdicts for every entity, so field-level
// redaction is a provable no-op under it; a policy-backed resolver may hide.
// Search uses this to decide whether the property-level filter needs to run at
// all (and thus whether a non-provenance searcher is acceptable).
func (svc affordanceService) hidesAnyField() bool {
	_, isNop := svc.resolver().(NopFieldVerdictResolver)
	return !isNop
}

// hiddenProperties returns the set of property names that should be
// stripped from v1.Entity.Properties before serialization. Caller uses
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

// redactedPropertyNames returns the sorted names of properties withheld by
// field-level ACL, for the `_redacted` wire field (DEC-T0XIWQ). Always
// non-nil — an empty slice is the closed-world "evaluated, nothing redacted"
// signal, distinct from `_redacted` being absent entirely (a shape that
// carries no write affordances, e.g. a list row).
//
// Takes already-resolved verdicts rather than re-resolving so a caller that
// has them (attachEntityAffordances) pays for one FieldVerdicts call, not two.
func redactedPropertyNames(v FieldVerdicts) []string {
	out := make([]string, 0, len(v.Visible))
	for name, visible := range v.Visible {
		if !visible {
			out = append(out, name)
		}
	}
	sort.Strings(out) // deterministic wire
	return out
}

// computeRelationAffordances returns the sparse `_relations` wire map
// for entity e. Only relation types with at least one deviation
// (creatable=false, removable=false, or any meta-field writable=false)
// appear in the map. Default-permissive types are absent — the SPA's
// "no entry = default" path handles them.
func (svc affordanceService) computeRelationAffordances(
	ctx context.Context, e *entityPkg.Entity,
) map[string]v1.RelationAffordance {
	v := svc.resolver().RelationVerdicts(ctx, e)
	out := make(map[string]v1.RelationAffordance)
	for relType, rv := range v.Types {
		var entry v1.RelationAffordance
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
		var fields map[string]v1.FieldAffordance
		for metaField, writable := range rv.Fields {
			if writable {
				continue
			}
			if fields == nil {
				fields = make(map[string]v1.FieldAffordance)
			}
			f := false
			fields[metaField] = v1.FieldAffordance{Writable: &f}
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
// surface (cards/list rows in v1.ViewEntity._props). Shallow copy: each
// value points at the same underlying object as e.Properties[k], which
// is fine because the response is JSON-marshaled before the caller can
// alias anything (TKT-IHC7D).
//
// Mirrors stripHiddenProperties's hidden-property contract but returns
// a new map instead of mutating an existing v1.Entity — the per-row
// case never goes through v1.Entity, so the in-place strip pattern
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
func (svc affordanceService) stripHiddenProperties(ctx context.Context, e *entityPkg.Entity, result *v1.Entity) {
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

// visibleRelationMeta returns a copy of a relation edge's property map with
// hidden meta keys removed, honoring the relation `visible:` grants resolved for
// the edge's SOURCE entity (TKT-B1F5Q1). meta is the raw edge property map; from
// is the relation's source entity (for incoming edges the peer, not the path
// entity; see [affordanceService.visibleRelationMetaIncoming]); relType is the canonical relation type.
//
// It NEVER mutates the argument: when at least one key is redacted it returns a
// fresh copy with those keys removed; when nothing is redacted it returns the
// input map unchanged (the store-owned edge.Properties, which some backends share
// across reads). Callers must therefore treat the result as read-only — the two
// live call sites only reassign it into the wire `rel["meta"]` and serialize,
// never mutate it. Returns the input unchanged when the active resolver does not
// implement [RelationVisibilityResolver] (Nop / Demo): relations then serialize
// un-redacted, matching pre-B1F5Q1 behavior. The deny universe is the meta map's
// actual keys, so redaction covers exactly what would reach the wire (including
// free-form keys not declared in the metamodel). Under a historical-subject ctx
// (relation history) the underlying resolver fails closed; a holder of
// history:read-redacted must skip this call entirely at the handler, not rely on
// it.
func (svc affordanceService) visibleRelationMeta(
	ctx context.Context, from *entityPkg.Entity, relType string, meta map[string]any,
) map[string]any {
	rv, ok := svc.resolver().(RelationVisibilityResolver)
	if !ok || len(meta) == 0 || from == nil {
		return meta
	}
	keys := make([]string, 0, len(meta))
	for k := range meta {
		keys = append(keys, k)
	}
	hidden := rv.RelationFieldVerdicts(ctx, from, relType, keys)
	if len(hidden) == 0 {
		return meta
	}
	out := make(map[string]any, len(meta))
	for k, v := range meta {
		if h, ok := hidden[k]; ok && !h {
			continue
		}
		out[k] = v
	}
	return out
}

// visibleRelationMetaIncoming redacts an INCOMING edge's meta, resolving the
// relation grant against the edge's true source (the peer's row at the edge's
// tail) rather than the entity being viewed (the TO side). It FAILS CLOSED if the peer cannot be
// fetched (RR-B1F5-N1): a peer deleted between the neighbor-visibility pass and
// this read would otherwise leave the source unresolvable, and falling back to
// the wrong-type path entity — whose type likely has no `visible:` block for this
// relation — would silently emit the meta un-redacted. When the active resolver
// can redact at all (implements [RelationVisibilityResolver]) and the source is
// unresolvable, drop the whole meta map rather than leak it. The write gate,
// [affordanceService.relationSources], resolves a missing tail row through the
// family instead; this read keeps the stricter rule, which only ever hides meta.
func (svc affordanceService) visibleRelationMetaIncoming(
	ctx context.Context, peer entityPkg.Ref, relType string, meta map[string]any,
) map[string]any {
	if len(meta) == 0 {
		return meta
	}
	if _, canRedact := svc.resolver().(RelationVisibilityResolver); !canRedact {
		return meta // Nop / Demo: no redaction at all, matching pre-B1F5Q1.
	}
	src, ok := svc.sourceRow(ctx, peer)
	if !ok {
		// Source gone mid-request → cannot resolve its grants → fail closed.
		return map[string]any{}
	}
	return svc.visibleRelationMeta(ctx, src, relType, meta)
}

// redactRelationMetaStrip reassigns one strip's `rel["meta"]` to the redacted copy
// (TKT-B1F5Q1) — the shared relation-meta redaction chokepoint both live handlers
// route through. The strip is the single source of truth for whether the edge is
// incoming: an outgoing edge resolves the grant against pathEntity; an incoming
// edge resolves against its peer (s.peer), failing closed if the peer is gone
// ([affordanceService.visibleRelationMetaIncoming]). Do NOT reintroduce an
// `incoming` parameter alongside s.incoming — a caller that disagreed with the
// strip could route an incoming edge down the outgoing branch and silently
// under-redact against the wrong-type pathEntity instead of failing closed
// (RR-B1F5-N3).
func (svc affordanceService) redactRelationMetaStrip(
	ctx context.Context, s relationMetaStrip, pathEntity *entityPkg.Entity, relType string,
) {
	meta, _ := s.rel["meta"].(map[string]any)
	if s.incoming {
		s.rel["meta"] = svc.visibleRelationMetaIncoming(ctx, s.peer, relType, meta)
		return
	}
	s.rel["meta"] = svc.visibleRelationMeta(ctx, pathEntity, relType, meta)
}

// computeTransitions returns the per-entity `_transitions` wire map: for each
// state-machine-typed property, the resolved outgoing transitions for the ctx
// principal on e. Returns nil (→ `_transitions` omitted from the wire) when the
// active resolver does not implement [TransitionResolver] (Nop / Demo) or has no
// machine-typed property for this entity — the SPA then falls back to the
// ordinary enum control. An empty (non-nil) map is possible for a
// TransitionResolver whose machines don't cover e's type; it serializes as
// `_transitions: {}`, the same closed-world "resolver present, no deviations"
// signal the other affordance maps use.
//
// fieldVerdicts is the same [FieldVerdicts] the caller resolved for `_fields` /
// `_attachments`; a property hidden by field-visibility policy is OMITTED here
// (RR-DENG8U). A machine's out-edge set is a function of its current value, so
// emitting `_transitions` for a hidden field would leak that value back through
// the wire past the strip that `stripHiddenProperties` performs on `properties`
// / `_title` — mirror the same boundary attachments already keep.
func (svc affordanceService) computeTransitions(
	ctx context.Context, e *entityPkg.Entity, fieldVerdicts FieldVerdicts,
) map[string][]v1.Transition {
	tr, ok := svc.resolver().(TransitionResolver)
	if !ok {
		return nil
	}
	verdicts := tr.TransitionVerdicts(ctx, e)
	out := make(map[string][]v1.Transition, len(verdicts))
	for prop, vs := range verdicts {
		if !fieldVerdicts.IsVisible(prop) {
			continue // hidden field: don't leak its state via out-edges
		}
		wire := make([]v1.Transition, 0, len(vs))
		for _, v := range vs {
			wire = append(wire, v1.Transition{
				To:      v.To,
				Label:   v.Label,
				Guard:   v.Guard,
				Allowed: v.Allowed,
				Reason:  string(v.Reason),
			})
		}
		out[prop] = wire
	}
	return out
}

// applyCreateLock adjusts a create-candidate response so every state-machine
// field is locked to its entry value (BUG-X1C7S / TKT-3G93B8). A create is an
// ENTRY, not a transition: the machine field must not be freely editable and
// must carry the initial value the server will accept. For each machine-typed
// property of entityType it (a) pins result.Properties[field] to the entry
// value, (b) marks it read-only in _fields so the SPA renders it locked and the
// create-commit filter omits it (the server then applies the initial), and (c)
// drops it from _transitions — offering "moves" on an entity that does not yet
// exist would be nonsensical, and the SPA keys the locked-field render off the
// entry, not off transitions.
//
// No-op when the resolver can't answer entry values (Nop / Demo) or the type has
// no machine field. Called only from the create dry-run path — GET / PATCH keep
// the normal editable + _transitions shape.
//
// A machine field hidden by field-visibility policy is skipped entirely
// (RR-C3OJ33): stripHiddenProperties already removed it from result.Properties,
// and re-inserting its entry value here would both re-leak it on the wire and
// make it re-appear in the SPA create form (which derives visible create fields
// from the response's property keys). Its create is still constrained — the
// server rejects a non-entry value on the real create regardless of the hint.
func (svc affordanceService) applyCreateLock(
	ctx context.Context, result *v1.Entity, candidate *entityPkg.Entity,
) {
	tr, ok := svc.resolver().(TransitionResolver)
	if !ok {
		return
	}
	entries := tr.EntryValues(candidate.Type)
	if len(entries) == 0 {
		return
	}
	fieldVerdicts := svc.fieldVerdicts(ctx, candidate)
	fields := map[string]v1.FieldAffordance{}
	if result.FieldAffordances != nil {
		fields = *result.FieldAffordances
	}
	if result.Properties == nil {
		result.Properties = map[string]any{}
	}
	for prop, entry := range entries {
		if !fieldVerdicts.IsVisible(prop) {
			continue // hidden: leave it stripped, don't re-add to the wire
		}
		result.Properties[prop] = entry
		locked := false
		fa := fields[prop]
		fa.Writable = &locked
		fields[prop] = fa
		if result.Transitions != nil {
			delete(*result.Transitions, prop)
		}
	}
	result.FieldAffordances = &fields
}

// attachEntityAffordances writes the per-entity `_fields` and
// `_relations` wire maps onto result. Called by paths that return a
// per-entity response (GET, PATCH, POST, clone, action) — list rows
// and includes get App.stripHiddenProperties only.
func (svc affordanceService) attachEntityAffordances(ctx context.Context, e *entityPkg.Entity, result *v1.Entity) {
	verdicts := svc.fieldVerdicts(ctx, e)
	fields := computeFieldAffordancesFrom(verdicts)
	relations := svc.computeRelationAffordances(ctx, e)
	result.FieldAffordances = &fields
	result.RelationAffordances = &relations
	// `_redacted` names what stripHiddenProperties removed, so a write surface
	// can tell "hidden" from "never set" instead of guessing from absence
	// (DEC-T0XIWQ). Rides the per-entity shapes only, like `_fields`.
	redacted := redactedPropertyNames(verdicts)
	result.Redacted = &redacted
	if transitions := svc.computeTransitions(ctx, e, verdicts); transitions != nil {
		result.Transitions = &transitions
	}
	if offers := svc.computeCopyOffers(ctx, e); offers != nil {
		result.Copies = &offers
	}
	if detail := svc.computeDetailActions(ctx, e); detail != nil {
		if result.Actions == nil {
			result.Actions = make(map[string]bool, len(detail))
		}
		maps.Copy(result.Actions, detail)
	}
	faces := svc.computeFaces(ctx, e)
	result.Faces = &faces
	// Pass the same verdicts so a policy-hidden `file` property's attachments
	// are omitted from `_attachments` — otherwise the hidden-field boundary
	// the rest of the response maintains would leak the file's metadata and a
	// working download href.
	// Bytes are keyed per entity, but a face lists only the names its own
	// value references (BUG-CTUW2N), so the hrefs carry the face address.
	attachments := svc.computeAttachments(ctx, e, result.Self, verdicts)
	result.Attachments = &attachments
}

// computeAttachments returns the per-property attachment metadata for an
// entity, keyed by `file`-type property name. Only properties that carry
// a file appear; an empty map means "no attachments". Rides every
// per-entity v1.Entity response (GET, PATCH, POST, clone) alongside
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
func (svc affordanceService) computeAttachments(
	ctx context.Context, e *entityPkg.Entity, selfHref string, verdicts FieldVerdicts,
) map[string][]v1.Attachment {
	out := make(map[string][]v1.Attachment)
	if selfHref == "" {
		return out
	}
	infos, err := svc.store.ListFamilyAttachments(ctx, e.ID)
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
	for _, info := range attachment.FaceAttachments(e, infos) {
		// A property hidden from this viewer by field-visibility policy must
		// not leak its files (metadata or a working download href) — mirror
		// the hidden-field boundary the rest of the response maintains.
		if !verdicts.IsVisible(info.Property) {
			continue
		}
		out[info.Property] = append(out[info.Property], v1.Attachment{
			ID:          info.FileName,
			FileName:    info.FileName,
			Size:        info.Size,
			ContentType: contentTypeForFilename(info.FileName),
			Href:        selfHref + "/_attachments/" + info.Property + "/" + url.PathEscape(info.FileName),
		})
	}
	return out
}

// computeFaces lists the entity's OTHER content states — the input to a
// "view the published face" / "go to draft" link.
//
// A face NAME is config (declared in schema.yaml, public). Whether THIS
// entity has that face, and whether this principal may read it, is data. Both
// come from [visibility.Resolver.Family], which runs the row gate and the
// per-face verdict a GET of each face runs, so a listed face always opens.
// Gating on the `type@face` grant alone is not enough: a face can be
// readable by one entity's owner edge and not another's.
//
// World-read is deliberately not checked. It is a GLOBAL, role-level grant
// (acl.Request.PermitsWorld takes a world name and nothing else) that the
// client already has from `/_schema`.worlds.
//
// It runs on per-entity responses only, never on list rows, so the one
// family read cannot become an N+1 over a page.
//
// Returns an empty (non-nil) slice when the entity has no other readable
// face, which is a real answer. That differs from the `_copies` convention,
// where nil means "capability not wired".
func (svc affordanceService) computeFaces(
	ctx context.Context, e *entityPkg.Entity,
) []v1.Face {
	out := []v1.Face{}
	if e == nil || svc.store == nil {
		return out
	}
	m := svc.meta()
	if m == nil {
		return out
	}
	def, ok := m.GetEntityDef(e.Type)
	if !ok {
		return out
	}
	if svc.family == nil || !otherFaceGranted(ctx, def, e) {
		return out
	}
	// The faces of e the principal may read: the resolver applies the row
	// gate and the per-face verdict, the same gates a GET of each face
	// runs. A face it withholds is not listed, so the menu never offers an
	// address that answers 404.
	fam, ok, err := svc.family(ctx, e.Type, e.ID)
	if err != nil {
		slog.Warn("dataentry: reading an entity's faces failed", "id", e.ID, "err", err)
		return out
	}
	if !ok {
		return out
	}
	for _, f := range fam.Faces {
		stored := f.String()
		if f == e.Face {
			continue // the face being served is not somewhere else to go
		}
		if _, declared := def.Faces[stored]; !declared {
			continue
		}
		// The operator's `label:` when declared, else the coordinate name.
		// Both are operator-authored config, so neither discloses anything
		// the schema endpoint does not already serve.
		//
		// The address is spelled with the DECLARED name so the bare face
		// gets an explicit form too (`POL-1@draft`), which is what makes it
		// literal under a world that would otherwise resolve `POL-1` away
		// from it. A bare face with no declared name has no such spelling.
		out = append(out, v1.Face{
			Face:  stored,
			Label: metamodel.FaceLabel(m, e.Type, stored),
			Ref:   faceRef(e, stored),
		})
	}
	// Sorted by the DECLARED name, not the label: the order must not shuffle
	// when an operator edits display text, and a label is optional so sorting
	// by it would interleave labeled and unlabeled faces arbitrarily.
	sort.Slice(out, func(i, j int) bool {
		return out[i].Face < out[j].Face
	})
	return out
}

// otherFaceGranted reports whether the principal's `type@face` grant admits
// any declared face of e's type other than the one served. Without one there
// is nothing to list, so computeFaces skips the family read: a faceless type,
// or a principal confined to one face, costs no store read.
func otherFaceGranted(ctx context.Context, def *metamodel.EntityDef, e *entityPkg.Entity) bool {
	for name := range def.Faces {
		if name != e.Face.String() && faceReadable(ctx, e.Type, entityPkg.Face(name)) {
			return true
		}
	}
	return false
}

// faceRef spells the explicit address of e's face at the stored coordinate:
// `ID@<declared name>`, or the bare id when the coordinate has no declared
// name. See [v1.Face.Ref].
func faceRef(e *entityPkg.Entity, stored string) string {
	declared := stored
	if declared == "" {
		return e.ID
	}
	return e.ID + entityPkg.StateRefSeparator + declared
}

// copyOffersFunc lists the copy affordances available from one face, as
// [entitymanager.CopyAffordances.CopiesForSource] does. A named type so the
// wiring site and the affordance service agree on one signature.
type copyOffersFunc func(
	ctx context.Context, entityType, face, sourceID string,
) ([]entitymanager.CopyOffer, error)

// computeCopyOffers lists the copy affordances available from e's current
// face — RULING 9's promote and translate buttons (TKT-F2D5U5).
//
// # Omitted, never silently empty
//
// Returns nil when the capability is not wired or the query fails, so
// `_copies` is OMITTED rather than sent as `[]`. An empty list is a legitimate
// answer ("this face declares no copies"), so emitting one for a missing
// capability would render a wiring gap as a domain fact — the visible symptom
// being "the promote button never appears", which sends someone hunting
// through schema.yaml for a definition that is correctly declared. A failure
// is logged for the same reason: an unlogged omission would look like "no
// copies here".
//
// The verdicts come from the kernel's own authorization path, not a
// re-derivation here, which is what stops this hint drifting from what the
// write actually does. This service must never compute invocability itself.
//
// The face is e's stored face: an affordance is offered FROM the face being
// viewed, so a promote appears on the draft and not on the published face.
func (svc affordanceService) computeCopyOffers(
	ctx context.Context, e *entityPkg.Entity,
) []v1.CopyOffer {
	if svc.copies == nil || e == nil {
		return nil
	}
	offers, err := svc.copies(ctx, e.Type, e.Face.String(), e.ID)
	if err != nil {
		slog.Warn("dataentry: computing copy offers failed; _copies omitted",
			"entity", e.ID, "type", e.Type, "err", err)
		return nil
	}
	out := make([]v1.CopyOffer, 0, len(offers))
	for _, o := range offers {
		out = append(out, v1.CopyOffer{
			Name:       o.Name,
			Label:      o.Label,
			TargetFace: o.TargetFace,
			Allowed:    o.Allowed,
			Reason:     o.Reason,
			OnSuccess:  copyOnSuccessWire(o.OnSuccess),
		})
	}
	return out
}

// copyOnSuccessWire projects a copy's declared follow-through onto the wire,
// nil when nothing was declared so an undeclared block is omitted rather than
// sent as an empty object. The landing's default is spelled out as
// `written`, so a client never infers it from an absent field.
func copyOnSuccessWire(s metamodel.CopyOnSuccess) *v1.CopyOnSuccess {
	if s.Message == "" && s.Landing.IsZero() {
		return nil
	}
	// The arms are mutually exclusive by construction — the loader refuses a
	// landing naming both a world and a face (validateCopyLanding) rather
	// than resolving it by precedence, and this projection must not quietly
	// introduce the precedence the loader declined to. So the arms test the
	// full shape, and an impossible combination falls to `written`.
	l := s.Landing
	landing := v1.CopyLanding{Mode: metamodel.LandingWritten}
	switch {
	case l.Mode != "":
		landing.Mode = l.Mode
	case l.World != "" && l.Face == "":
		landing.Mode = "world"
		landing.World = l.World
	case l.Face != "" && l.World == "":
		landing.Mode = "face"
		landing.Face = l.Face
	}
	return &v1.CopyOnSuccess{Message: s.Message, Landing: landing}
}
