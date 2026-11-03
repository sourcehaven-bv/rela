package aclmap

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Verbs returns the verbs the access reports answer, in [acl.Verbs] order:
// every verb except rename. The grant helpers behind the reports read the
// `update:` list for rename, while the runtime asks for a `rename:` grant on a
// faced type and also honors one on a faceless type, so a rename report would
// be wrong in both directions. TKT-4APGVF fixes the helpers and drops this
// exclusion. A new write Op appears here without a change.
func Verbs() []acl.Verb {
	return slices.DeleteFunc(acl.Verbs(), func(v acl.Verb) bool { return v == acl.VerbRename })
}

// checkReportable returns an error unless verb is one of [Verbs]. Rename
// gets its own message: it is a known verb that the reports leave out until
// TKT-4APGVF, not a typo.
func checkReportable(verb acl.Verb) error {
	if slices.Contains(Verbs(), verb) {
		return nil
	}
	if verb == acl.VerbRename {
		return fmt.Errorf("aclmap: verb %q is not reported yet (TKT-4APGVF)", verb)
	}
	return fmt.Errorf("aclmap: unknown verb %q", verb)
}

// resolveVerbs turns a --verb filter into the ordered verb list to
// cover: the single verb when set (validated), else every verb in [Verbs].
func resolveVerbs(verbFilter acl.Verb) ([]acl.Verb, error) {
	if verbFilter == "" {
		return Verbs(), nil
	}
	if err := checkReportable(verbFilter); err != nil {
		return nil, err
	}
	return []acl.Verb{verbFilter}, nil
}

func verbStrings(verbs []acl.Verb) []string {
	out := make([]string, len(verbs))
	for i, v := range verbs {
		out[i] = string(v)
	}
	return out
}

// resolveEffective maps a raw principal key to its effective identity
// (resolved user-entity ID when principal_property is configured, else
// the raw key), returning the effective user and the raw value to show
// when they differ. The raw key is trimmed first, matching who-can's
// accessFor, so the two "same read path" commands agree on a principal
// typed with stray whitespace.
func (e *Engine) resolveEffective(ctx context.Context, raw string) (user, rawShown string, err error) {
	raw = strings.TrimSpace(raw)
	if id, rErr := e.resolver.IdentityResolver().ResolvePrincipal(ctx, raw); rErr != nil {
		return "", "", rErr
	} else if id != "" && id != raw {
		return id, raw, nil
	}
	return raw, "", nil
}

// TypeAccess is one principal's effective access to one entity type,
// split into a type-level Baseline (grants that apply to EVERY entity of
// the type — global assignments, group roles, everyone) and per-entity
// Exceptions (entities that carry ADDITIONAL routes via a graph edge or
// inheritance, so they differ from the baseline).
//
// The split is the scale mechanism: a project with thousands of entities
// of a type collapses to one baseline plus only the handful of entities
// where the graph grants something extra. An entity whose access equals
// the baseline is never enumerated.
type TypeAccess struct {
	Type string `json:"type"`
	// Baseline maps each verb the principal holds type-wide to the routes
	// that confer it. A verb absent from the map is not granted type-wide
	// (it may still appear on an Exception).
	Baseline map[string][]Route `json:"baseline,omitempty"`
	// Exceptions are entities of this type whose access exceeds the
	// baseline, sorted by entity ID.
	Exceptions []EntityException `json:"exceptions,omitempty"`
	// Faces maps a declared face of the type to the verbs a global role
	// grants on that face only (`type@face` in acl.yaml), sorted. A verb the
	// baseline already holds type-wide is not repeated here. Face grants that
	// arrive through an entity-local role are not listed.
	Faces map[string][]string `json:"faces,omitempty"`
}

// EntityException is one entity that grants the principal MORE than its
// type baseline — the extra access, per verb, with routes. Only the
// surplus over the baseline is recorded; baseline-equal verbs are
// omitted so the exception shows exactly what is special about this
// entity.
type EntityException struct {
	Entity string `json:"entity"`
	// Extra maps each verb granted here-but-not-in-baseline (or granted by
	// an additional route beyond the baseline) to the surplus routes.
	Extra map[string][]Route `json:"extra"`
}

// MapPrincipalResult is the answer to "what can principal P do, and
// where". It fixes the principal and varies the entity — the inverse of
// [WhoCanResult]. SchemaVersion tracks the same wire contract.
type MapPrincipalResult struct {
	SchemaVersion int    `json:"schema_version"`
	Principal     string `json:"principal"`
	Raw           string `json:"raw,omitempty"`
	// Verbs is the verb set this map covers (every verb, or the single
	// filtered verb).
	Verbs []string `json:"verbs"`
	// Types is the per-type access, sorted by type name. A type with no
	// baseline and no exceptions for the principal is omitted.
	Types []TypeAccess `json:"types"`
	// EveryoneOnly is true when the principal has NO grant beyond what the
	// built-in everyone role gives every principal — the "fully cut off"
	// signal for offboarding (UC2).
	EveryoneOnly bool `json:"everyone_only"`

	// As and Scopes echo the client attenuation this map was computed under
	// (TKT-IAC8TX), empty when none was requested. Echoed rather than left
	// implicit because the same principal yields DIFFERENT maps through
	// different clients — an artifact that did not say which client it
	// described would be actively misleading.
	As     string   `json:"as,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
	// Baseline names the client baseline As matched. Empty when As is set
	// and no baseline lists it, which leaves the client unrestricted.
	Baseline string `json:"baseline,omitempty"`

	// Webhook is the hook id when this maps a webhook delivery
	// ([Engine.MapWebhook]); As is then always "webhook".
	Webhook string `json:"webhook,omitempty"`
	// WebhookCheck compares the hook's mapping with its access, when the
	// caller passed the mapping to [Engine.MapWebhook].
	WebhookCheck *WebhookCheck `json:"webhook_check,omitempty"`
	// Permissions and Relations are filled only when [Extras] asks for them:
	// the named permissions held, and the relation types the principal may
	// write. Both are omitted from JSON when empty.
	Permissions []string         `json:"permissions,omitempty"`
	Relations   []RelationAccess `json:"relations,omitempty"`
}

// ClientView asks for a map computed as a client of the given principal_type,
// carrying the given scopes — what `rela acl map --as app` reports.
//
// The zero value means "no attenuation": the map covers the principal's own
// access, which is what every caller predating client attenuation gets.
type ClientView struct {
	PrincipalType string
	Scopes        []string
}

// isZero reports whether no client view was requested.
func (c ClientView) isZero() bool {
	return c.PrincipalType == "" && len(c.Scopes) == 0
}

// MapPrincipal computes principal P's effective access across the given
// entity types, aggregated by type with per-entity exceptions (UC1
// onboarding verification, UC2 offboarding review).
//
// entityTypes is the type universe to cover — the caller supplies it
// (from the metamodel) so the engine keeps no metamodel dependency;
// typeFilter, when non-empty, narrows it to that one type. verbFilter,
// when non-empty, restricts the map to that single verb; empty covers all
// four.
//
// Read is decided by the same runtime read path as [Engine.WhoCan]
// (acl.Request.AccessRoutes gates read on PermitsRead), so the
// per-principal view carries the same no-false-negative guarantee.
//
// Cost is O(types × entities-per-type) resolver probes for one principal,
// with the member-of walk memoised across the single Request. Acceptable
// for one principal; the whole-graph map (all principals) is a later
// slice that will need a reverse index.
func (e *Engine) MapPrincipal(
	ctx context.Context, rawPrincipal string, verbFilter acl.Verb, typeFilter string, entityTypes []string,
) (*MapPrincipalResult, error) {
	return e.MapPrincipalAs(ctx, rawPrincipal, verbFilter, typeFilter, entityTypes, ClientView{})
}

// MapPrincipalAs is [Engine.MapPrincipal] computed as a CLIENT of the
// principal — the answer to "what can this MCP do when Alice connects it?"
// (TKT-IAC8TX).
//
// A zero view is exactly [Engine.MapPrincipal]. A non-zero one resolves the
// principal's client ceiling, so the reported access is what the client
// actually has: the principal's own grants intersected with the baseline its
// principal_type selects, widened by the scopes it presents.
//
// This is why the attenuation claims are stamped through principal.Verified
// here rather than assembled as a literal: they are unexported precisely so no
// composite literal can populate them, and an attestation tool must go through
// the same door the request path does or it reports a different policy than the
// one that runs.
func (e *Engine) MapPrincipalAs(
	ctx context.Context, rawPrincipal string, verbFilter acl.Verb, typeFilter string,
	entityTypes []string, view ClientView,
) (*MapPrincipalResult, error) {
	return e.MapPrincipalFull(ctx, rawPrincipal, verbFilter, typeFilter, entityTypes, view, Extras{})
}

// typeAccess computes the principal's access to one entity type: the
// type-level baseline (grants that apply to EVERY entity of the type —
// everyone, asserted, global assignment, group role) and per-entity
// exceptions (entities that carry ADDITIONAL local / inherited routes).
//
// The baseline is computed FIRST, entity-independently — it does NOT
// depend on iterating entities. This is load-bearing: a global/group
// grant on a type with ZERO entities must still show (and must NOT read
// as "cut off" — the offboarding false all-clear, RR fixed here). Sources:
//
//   - everyone / asserted: pure policy lookups (EveryoneGrants /
//     AssertedGrants). AccessRoutes omits the everyone role, so it is
//     seeded explicitly.
//   - global / group: AccessRoutes(verb, type, "") — the empty-entity
//     probe returns the principal's Globals-only attributions (no local
//     edges, since there is no entity), i.e. exactly the type-level set.
//
// Only THEN are entities iterated, and only to find LOCAL / inherited
// routes beyond the baseline — those are the per-entity exceptions. Any
// type-level kind reaching the exception path is a classification bug and
// panics (isTypeLevel is an assertion, not a partition).
//
// sawNonEveryone is true if the principal has any grant beyond the
// everyone role (baseline global/group OR an exception) — the signal that
// clears MapPrincipalResult.EveryoneOnly. Asserted grants do NOT clear it:
// their holders live in the IdP, so this principal may not hold the claim.
func (e *Engine) typeAccess(
	ctx context.Context, req *acl.Request, typ string, verbs []acl.Verb,
) (TypeAccess, bool, error) {
	ta := TypeAccess{Type: typ}

	baseline, baseNonEveryone, err := e.typeBaseline(ctx, req, typ, verbs)
	if err != nil {
		return TypeAccess{}, false, err
	}
	exceptions, exNonEveryone, err := e.typeExceptions(ctx, req, typ, verbs)
	if err != nil {
		return TypeAccess{}, false, err
	}

	ta.Faces = e.splitFaces(req, typ, verbs, baseline)
	if len(baseline) > 0 {
		ta.Baseline = baseline
	}
	ta.Exceptions = exceptions
	return ta, baseNonEveryone || exNonEveryone, nil
}

// splitFaces decides a faced type per declared face, the way the runtime
// does, and returns the faces each verb is held on when it is not held on
// every face.
//
// The baseline is decided type-granularly and so overstates a faced type in
// two ways: a face-scoped read grant (`read: [policy@nl]`) counts as a read
// of the whole type, and a bare or "*" write grant counts although on a write
// it covers only the unnamed face, which a faced type does not have. For a
// faced type, splitFaces therefore keeps a baseline verb only when it is held
// on every declared face, removes it otherwise, and reports the faces it is
// held on. Without this, an expectations file could pass while the
// runtime allows or refuses a face-scoped write.
//
// The roles asked are the global roles (which include the everyone role) and,
// unless the principal is a webhook, the asserted roles. The answer goes
// through [acl.Request.RoleGrantsVerbOnFace], which applies the client
// ceiling first. A faceless type returns nil and leaves baseline unchanged.
func (e *Engine) splitFaces(
	req *acl.Request, typ string, verbs []acl.Verb, baseline map[string][]Route,
) map[string][]string {
	faces := e.faces(typ)
	if len(faces) == 0 {
		return nil
	}
	out := map[string][]string{}
	for _, verb := range verbs {
		var held []entity.Face
		for _, face := range faces {
			if e.heldOnFace(req, verb, typ, face) {
				held = append(held, face)
			}
		}
		if _, typeWide := baseline[string(verb)]; typeWide && len(held) == len(faces) {
			continue
		}
		// Held on some faces, or on every face through face grants that the
		// type-granular baseline does not see: report the faces.
		delete(baseline, string(verb))
		for _, face := range held {
			out[string(face)] = append(out[string(face)], string(verb))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// heldOnFace reports whether a type-level role of the principal grants verb
// on face of typ. See splitFaces for the roles asked.
func (e *Engine) heldOnFace(req *acl.Request, verb acl.Verb, typ string, face entity.Face) bool {
	for _, a := range req.Globals().Attributions {
		if req.RoleGrantsVerbOnFace(a.Role, verb, typ, face) {
			return true
		}
	}
	if principal.IsWebhookUser(req.Principal().User) {
		return false
	}
	for _, ag := range acl.ClampedAssertedGrants(req, verb, typ) {
		if req.RoleGrantsVerbOnFace(ag.Role, verb, typ, face) {
			return true
		}
	}
	return false
}

// typeBaseline computes the entity-INDEPENDENT grants for a type: the
// everyone role, asserted-claim grants, and the principal's global/group
// assignments (via the empty-entity probe, which yields Globals-only
// attributions). Returns the per-verb route map and whether any non-
// everyone route was seen. This does NOT touch entities, so a global/
// group grant shows even when the type has zero entities.
func (e *Engine) typeBaseline(
	ctx context.Context, req *acl.Request, typ string, verbs []acl.Verb,
) (routes map[string][]Route, sawNonEveryoneOut bool, err error) {
	baseline := map[string][]Route{}
	seen := map[string]struct{}{}
	var sawNonEveryone bool
	// A webhook delivery never carries IdP claims, so no asserted role can
	// reach it; reporting them would attest access the runtime never grants.
	claimless := principal.IsWebhookUser(req.Principal().User)
	for _, verb := range verbs {
		// The clamped forms, not the policy-wide ones on the resolver: this
		// map is about one principal, and under a client ceiling the everyone
		// role and asserted roles are narrowed like every other role.
		if eg := acl.ClampedEveryoneGrants(req, verb, typ); eg.Granted {
			addBaseline(baseline, seen, string(verb),
				Route{Kind: acl.SourceGlobal.String(), Role: acl.EveryoneRole})
		}
		var asserted []acl.AssertedGrant
		if !claimless {
			asserted = acl.ClampedAssertedGrants(req, verb, typ)
		}
		for _, ag := range asserted {
			addBaseline(baseline, seen, string(verb),
				Route{Kind: "asserted", Role: ag.Role, Claim: ag.Claim})
		}
		attrs, err := req.AccessRoutes(ctx, verb, typ, "")
		if err != nil {
			return nil, false, fmt.Errorf("aclmap: type-level routes for %s: %w", typ, err)
		}
		for _, a := range attrs {
			assertTypeLevel(a.Source.Kind, typ) // Globals-only must be type-level
			sawNonEveryone = true
			addBaseline(baseline, seen, string(verb), routeFromAttribution(a))
		}
	}
	for v := range baseline {
		sort.Slice(baseline[v], func(i, j int) bool { return lessRoute(baseline[v][i], baseline[v][j]) })
	}
	return baseline, sawNonEveryone, nil
}

// typeExceptions iterates the type's entities and collects the LOCAL /
// inherited routes (beyond the type baseline) that make an entity differ.
// Type-level kinds are skipped (already in the baseline). Returns the
// sorted exception list and whether any (necessarily non-everyone) route
// was seen.
func (e *Engine) typeExceptions(
	ctx context.Context, req *acl.Request, typ string, verbs []acl.Verb,
) ([]EntityException, bool, error) {
	var exceptions []EntityException
	var sawNonEveryone bool
	// Every face: access routes are decided per id, and a faced entity has
	// no default row. Rows arrive ordered by id, so a repeated id is the
	// same family and is decided once.
	var last string
	for ent, err := range store.ListEntityHeaders(ctx, e.src, store.EntityQuery{Type: typ, Faces: store.AllFaces()}) {
		if err != nil {
			return nil, false, fmt.Errorf("aclmap: list %s entities: %w", typ, err)
		}
		if ent.ID == last {
			continue
		}
		last = ent.ID
		extra, saw, err := e.entityExtra(ctx, req, typ, ent.ID, verbs)
		if err != nil {
			return nil, false, err
		}
		sawNonEveryone = sawNonEveryone || saw
		if len(extra) > 0 {
			exceptions = append(exceptions, EntityException{Entity: ent.ID, Extra: extra})
		}
	}
	sort.Slice(exceptions, func(i, j int) bool { return exceptions[i].Entity < exceptions[j].Entity })
	return exceptions, sawNonEveryone, nil
}

// entityExtra returns the local / inherited routes for one entity, per
// verb — the surplus over the type baseline.
func (e *Engine) entityExtra(
	ctx context.Context, req *acl.Request, typ, entityID string, verbs []acl.Verb,
) (routes map[string][]Route, sawNonEveryoneOut bool, err error) {
	extra := map[string][]Route{}
	var sawNonEveryone bool
	for _, verb := range verbs {
		attrs, err := req.AccessRoutes(ctx, verb, typ, entityID)
		if err != nil {
			return nil, false, fmt.Errorf("aclmap: access routes for %s: %w", entityID, err)
		}
		for _, a := range attrs {
			if isTypeLevel(a.Source.Kind) {
				continue // captured in the baseline, not an exception
			}
			sawNonEveryone = true
			extra[string(verb)] = append(extra[string(verb)], routeFromAttribution(a))
		}
	}
	for v := range extra {
		sort.Slice(extra[v], func(i, j int) bool { return lessRoute(extra[v][i], extra[v][j]) })
	}
	return extra, sawNonEveryone, nil
}

// isTypeLevel reports whether a source kind applies to EVERY entity of a
// type — a global assignment, a group role, or an asserted-claim grant —
// rather than being conferred by an edge to a specific entity or its
// ancestor. It partitions the closed acl.SourceKind enum: the three
// entity-independent kinds are type-level; the four Local* kinds are
// entity-specific.
func isTypeLevel(k acl.SourceKind) bool {
	switch k {
	case acl.SourceGlobal, acl.SourceGroup, acl.SourceAsserted:
		return true
	case acl.SourceLocal, acl.SourceLocalViaGroup,
		acl.SourceLocalViaAncestor, acl.SourceLocalViaGroupAndAncestor:
		return false
	default:
		return false
	}
}

// assertTypeLevel panics if kind is not type-level. It guards the
// empty-entity baseline probe, whose attributions come from Globals only
// (global / group / asserted) and can never be a local-edge route. A
// non-type-level kind here would mean the resolver's Globals contract
// changed underneath us — fail loud in tests rather than silently emit a
// per-entity route as a type-wide baseline.
func assertTypeLevel(k acl.SourceKind, typ string) {
	if !isTypeLevel(k) {
		panic(fmt.Sprintf("aclmap: non-type-level source kind %v from empty-entity probe on %q "+
			"(Globals contract violated)", k, typ))
	}
}

// addBaseline records a type-level route under verb once (deduped across
// the entities that all exhibit it). The key spans every Route field that
// distinguishes a type-level route; Ancestor is included for completeness
// even though a type-level route never carries one (isTypeLevel excludes
// the Local*Ancestor kinds), so a future misclassification collides
// loudly in tests rather than silently dropping a route.
func addBaseline(baseline map[string][]Route, seen map[string]struct{}, verb string, r Route) {
	key := verb + "\x00" + r.Kind + "\x00" + r.Role + "\x00" + r.Group +
		"\x00" + r.Ancestor + "\x00" + r.Relation + "\x00" + r.Claim
	if _, dup := seen[key]; dup {
		return
	}
	seen[key] = struct{}{}
	baseline[verb] = append(baseline[verb], r)
}

// routeFromAttribution is the single-attribution form of
// routesFromAttributions.
func routeFromAttribution(a acl.RoleAttribution) Route {
	return Route{
		Kind:     a.Source.Kind.String(),
		Role:     a.Role,
		Group:    a.Source.Group,
		Ancestor: a.Source.Ancestor,
		Relation: a.Source.Relation,
		Claim:    a.Source.Claim,
	}
}

// mapTypes returns the sorted, de-duplicated entity-type names to cover:
// the single typeFilter when set, else every type in entityTypes.
func mapTypes(typeFilter string, entityTypes []string) []string {
	if typeFilter != "" {
		return []string{typeFilter}
	}
	seen := make(map[string]struct{}, len(entityTypes))
	out := make([]string, 0, len(entityTypes))
	for _, t := range entityTypes {
		if _, dup := seen[t]; dup || t == "" {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}
