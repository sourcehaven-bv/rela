package entitymanager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"strings"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/computed"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
)

// TemplateLoader is the narrow consumer-side surface entitymanager
// needs from a templating implementation. The full
// [templating.Templater] interface has five methods; Manager calls
// exactly two. Defining this interface here (CLAUDE.md
// "consumer-side interfaces" rule) lets tests stub two methods
// instead of five, and keeps Manager decoupled from the
// template-generation half of templating's API.
type TemplateLoader interface {
	EntityTemplate(ctx context.Context, entityType, variant string) (*templating.Template, error)
	RelationTemplate(ctx context.Context, relationType string) (*templating.Template, error)
}

// Manager is the production entity-manager implementation — the whole of this
// package's write API. It runs metamodel validation, automation rules (via
// [automation.Engine]), and dispatches automation cascades through an
// [autocascade.Runner].
//
// Manager is constructed at each per-command wiring site (cmd/rela,
// cmd/rela-server, cmd/rela-desktop, plus subcommands that need their own
// write path). Consumers depend on a scoped consumer-side interface declared
// in their OWN package, never on a wide one declared here (see CLAUDE.md and
// the package doc): appbuild hands out *Manager, and each consumer's
// interface is satisfied structurally. The wide package-level EntityManager
// interface that used to sit beside this type was removed in TKT-IVSJV6.
//
// Pipeline shapes preserved from the pre-decomposition workspace
// implementation (see PLAN-HQ5Y):
//
//   - Create: createCore (validate → write) → automation.Process →
//     apply property changes → re-write if changed → cascade.
//     Engine.Process runs here (not inside Runner) because
//     PropertiesSet must land on the entity and be persisted before
//     cascade dispatches.
//   - Update: validate → engine.Process(if oldEntity != nil) →
//     apply property changes → write → cascade.
//   - Delete: lookup → collect incident relations → delete relations
//     → delete entity. No automation, no cascade.
//   - Rename: validate → write at new ID → rewrite incident relations
//     → delete old. No automation, no cascade, no re-validation.
//   - CreateRelation: fetch endpoints → validate type → check duplicate
//     → apply template → write. No automation.
//   - UpdateRelation: fetch existing → merge properties → MetaUnset →
//     content → write. No automation.
//   - DeleteRelation: delete. No automation.
//
// +1 (BUG-64MU2Q): DeleteRelationState, the face-addressed form of
// DeleteRelation. It cannot be an option on the existing method — the tail is
// part of a relation's identity, so the two address different edges, which is
// why the store draws the same line between DeleteRelation and
// DeleteRelationState. Pinned here rather than split: the pair belongs on the
// type that owns the write path.
//
// -4 (TKT-UA1W1L): removing sync dropped ApplyEntity, ApplyRelation and
// their two helpers.
//
//plimsoll:max-methods=37
type Manager struct {
	deps Deps

	// bypassACL marks this Manager as an ELEVATED write handle: its writes
	// skip the ACL deny (TKT-D8T148, the `rela.bypass_acl(...)` path). It is
	// false on the normal Manager and set true ONLY on the throwaway handle
	// returned by Manager.elevated. Carrying elevation on the object (not
	// the context) is what makes it leak-proof: a nested cascade an elevated
	// write triggers re-dispatches with the gated Manager (see the cascade
	// dispatch sites, which pass `m.gated()` as the Mutator), so elevation
	// never propagates to descendant writes.
	bypassACL bool

	// cascadeWrite marks a handle an automation cascade writes through
	// (cascadeHost). Its writes were authorized by the write that triggered
	// the cascade, so authorizeAndAudit neither consults the ACL nor records
	// a bypass: every cascadeHost write has always been ungated, and a bypass
	// row per cascade step would misreport an ordinary automation as an
	// elevated one. Set only by cascadeHost, on a throwaway handle.
	cascadeWrite bool

	// fieldGate marks a handle whose caller-authored property writes pass
	// [Deps.FieldGate]. Set only by [FieldGated].
	fieldGate bool
}

// elevated returns a throwaway Manager handle whose writes skip the ACL deny.
// Shares all deps with the receiver; differs only in bypassACL. Reserved for
// the script-runner's elevated write handle (rela.bypass_acl). NOT exported as
// a general capability — callers obtain it only through the autocascade
// Mutator seam for an allow_acl_bypass action.
func (m *Manager) elevated() *Manager {
	return &Manager{deps: m.deps, bypassACL: true}
}

// Elevated satisfies [autocascade.ElevatedProvider] (TKT-D8T148): it hands the
// script runner an elevated Mutator for an `allow_acl_bypass` action. Exported
// only to cross the package boundary to the script runner; the returned handle
// bypasses the ACL deny but preserves the real principal in audit and never
// propagates elevation into nested cascades (it dispatches via gated()).
func (m *Manager) Elevated() autocascade.Mutator {
	return m.elevated()
}

// gated returns a non-elevated Manager sharing the receiver's deps. On a normal
// Manager it returns the receiver; on an elevated or cascade-writer handle it
// strips the bypass. Used at cascade-dispatch sites so a nested cascade
// triggered by an elevated write runs with normal ACL authority — elevation
// does not propagate to descendants (the leak the ctx-marker approach would
// have had).
func (m *Manager) gated() *Manager {
	if !m.bypassACL && !m.cascadeWrite && !m.fieldGate {
		return m
	}
	return &Manager{deps: m.deps}
}

// FieldGated returns a handle on m's deps whose caller-authored property
// writes pass [Deps.FieldGate] (TKT-0XL8MF). Hand it to a surface that
// writes for a principal and has no field check of its own: MCP on
// rela-server and scheduled Lua. The default handle skips the gate, because
// its callers either check fields themselves (the data-entry API's
// validateFieldWrite) or act for the operator or the system (the CLI,
// provisioning, CalDAV mapping, operator-declared actions).
//
// The handle does not pass its gating to an automation cascade it triggers:
// cascades dispatch through gated(), which drops it, so automation output
// stays ungated (RR-00ERM9). An elevated handle skips the gate like the row
// ACL (RR-BA1NIV).
//
// Nil: rejected (panics); wiring hands it the manager it built.
func FieldGated(m *Manager) *Manager {
	return &Manager{deps: m.deps, fieldGate: true}
}

// fieldGated reports whether writes through m pass the [FieldWriteGate].
func fieldGated(m *Manager) bool {
	return m.fieldGate && !m.bypassACL && !m.cascadeWrite
}

// auditedDenial is a refusal that writes its own audit summary.
// affordances.FieldWriteError is one.
type auditedDenial interface {
	error
	AuditSummary() string
}

// checkFieldWrite asks the [FieldWriteGate] and records a refusal as a
// denied write, like a row-level refusal (recordDeniedWrite). Without the
// record, a caller probing read-only fields over MCP or Lua would leave no
// trace. A plain function: Manager is at its method cap.
func checkFieldWrite(ctx context.Context, m *Manager, e *entity.Entity, set map[string]any, unset []string) error {
	err := m.deps.FieldGate.CheckFieldWrite(ctx, e, set, unset)
	if err == nil || isAffordanceProbe(ctx) {
		return err
	}
	summary := "denied: " + err.Error()
	var d auditedDenial
	if errors.As(err, &d) {
		summary = d.AuditSummary()
	}
	m.deps.Audit.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpDeniedWrite,
		Subject:     &audit.Subject{Kind: "entity", Type: e.Type, ID: e.ID},
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     summary,
	})
	return err
}

// Compile-time assertion: Manager must satisfy the autocascade.Mutator
// surface (the per-cascade write handle scripted actions receive), so a
// drift surfaces at this type rather than at the call site that passes
// Manager into Request.Mutator.
//
// There is deliberately no assertion against a package-local write
// interface: the wide EntityManager one was deleted in TKT-IVSJV6, and its
// replacements are declared at each CONSUMER (lua.Mutator,
// attachment.Stamper, mcp.EntityWriter, and the unexported ones in
// internal/cli and internal/dataentry). Asserting against them here would
// re-import every consumer and reinstate the coupling the split removed;
// each is checked where it is used, at its own wiring site.
var _ autocascade.Mutator = (*Manager)(nil)

// Deps is the constructor input for [New]. Using a struct keeps the
// constructor signature stable as new collaborators land (audit,
// principal, policy in subsequent tickets).
type Deps struct {
	// Store is the authoritative persistence layer. Required.
	Store store.Store

	// Meta is the active metamodel. Manager uses it for
	// ValidateEntity (entity writes) and ValidateRelation (relation
	// writes). Required.
	Meta *metamodel.Metamodel

	// Templater applies entity-creation and relation-creation
	// templates. The full [templating.Templater] satisfies this
	// narrow contract structurally. Required (use a no-op in tests).
	Templater TemplateLoader

	// Automations is the rule-evaluation engine. Manager calls it
	// on EventEntityCreated / EventEntityUpdated to discover side
	// effects. Optional: nil disables automation processing.
	Automations *automation.Engine

	// Cascade is the autocascade Runner that orchestrates automation
	// side effects after a write. Required iff Automations is non-nil.
	Cascade *autocascade.Runner

	// ScriptRunner executes scripted automation actions during a
	// cascade. May be nil if no scripted automations are configured;
	// Runner records each scripted action as a per-action error when
	// no ScriptRunner is supplied. Wiring sites that need
	// transport-specific deps (e.g. Lua) construct one with the
	// static read deps at this layer; the per-cascade mutator is
	// supplied by Manager via [autocascade.Request.Mutator] (see
	// internal/script/luascriptrunner.go for the Lua adapter).
	ScriptRunner autocascade.ScriptRunner

	// Audit receives one record per successful entity / relation write.
	// Required. Production wiring passes a [audit.Filesystem]; tests use
	// [audit.NewMemory] or [audit.Nop]. Never substitute a silent nil —
	// the constructor rejects nil so missing audit fails fast at wiring
	// time, not later as silently-dropped forensic data.
	Audit audit.Audit

	// ACL gates every write entry point. Required. Production wiring
	// passes [acl.NopACL] (no acl.yaml) or [acl.Declarative] (acl.yaml
	// present); `rela-server --read-only` injects [acl.ReadOnlyACL].
	// Tests use [acl.NopACL] unless they assert on the deny path.
	// Never substitute a silent nil — the constructor rejects nil so
	// missing ACL fails fast at wiring time, not later as a silently
	// disabled authz gate.
	ACL acl.ACL

	// VersionRecorder captures a synchronous entity version for rename and
	// delete (see [VersionRecorder]). Optional: nil disables synchronous
	// version capture (fs/mem builds, and the postgres build's create/update
	// versions are handled by the store's periodic sweep, not this hook).
	VersionRecorder VersionRecorder

	// AliasRewriter is notified when an entity is renamed or deleted so a
	// subsystem holding references BY ENTITY ID (the CalDAV alias service) can
	// rewrite them (see [AliasRewriter]). Optional: nil disables the hook.
	AliasRewriter AliasRewriter

	// RelationVersionRecorder captures a synchronous relation version for
	// relation delete (explicit and entity-cascade) and endpoint rename (see
	// [RelationVersionRecorder]). Optional: nil disables it (fs/mem builds; the
	// postgres build's relation create/update versions come from the sweep).
	RelationVersionRecorder RelationVersionRecorder

	// Transitions enforces enum state machines on the write path (TKT-E4LW2):
	// transition legality, guard permissions, and preconditions. Required so
	// no write path can silently skip the machine — but a metamodel with no
	// transitions compiles to an empty enforcer whose checks are no-ops, so
	// "required" costs nothing when the feature is unused. Constructed once at
	// startup by [statemachine.Compile] and injected; the Manager never
	// re-derives it from the metamodel.
	Transitions TransitionEnforcer

	// Computed is the compiled materialized-property evaluator. If omitted,
	// New compiles it from Meta so a wiring omission can never skip
	// computation. Production appbuild injects the already-compiled set.
	Computed *computed.Set

	// TransitionGuard answers the guard question for a state-machine
	// transition (does the ctx principal hold permission P for the subject).
	// May be nil: a nil guard makes every guarded edge fail closed, which is
	// the safe default when no ACL-backed guard was wired. Production wiring
	// supplies an adapter over the ACL; the direct-CLI/no-policy case is
	// handled inside that adapter (it allows when there is no policy).
	TransitionGuard statemachine.Guard

	// TransitionGraph answers has_relation/count_relations for a transition
	// `when:` precondition. May be nil when no precondition needs the graph;
	// a `when:` that queries the graph then evaluates against an empty graph.
	TransitionGraph statemachine.GraphLookup

	// FieldGate decides whether the ctx principal may author the specific
	// property changes in a [Manager.PatchEntity] call. Required — pass
	// [AllowAllFieldGate] to opt out explicitly.
	//
	// It is a CAPABILITY injected at the wiring site, never inferred from
	// identity (the write-side analog of the read-side AllowAllReader,
	// DEC-ZBI39P). Operator-trust-boundary entry points (CLI) wire
	// AllowAllFieldGate because they have full access by design;
	// request-scoped surfaces are where a policy-backed implementation
	// belongs — none is wired yet (TKT-0XL8MF); see [FieldWriteGate].
	//
	// Required rather than optional-nil on purpose: a silently-nil authz
	// gate is the "forgotten wiring must not become an ACL bypass" failure
	// (RR-X9NVHI), and it matches how ACL and Audit are already handled.
	FieldGate FieldWriteGate

	// CopyVisibility is the CALLER'S read gate, used only by CROSS-ENTITY
	// copies (TKT-C1XUA8, design doc §9.2).
	//
	// Deliberately NOT used by same-entity copies: those run elevated,
	// because hidden fields travel with the entity and the same policy
	// governs them on the target face. Routing them through this gate would
	// be the redacted-read-feeds-a-write bug, which destroys the fields the
	// principal could not see.
	//
	// Nil: accepted without a policy — the read is then raw, which is what
	// every other read on such a deployment does, and is what the CLI passes.
	// REJECTED when ACL is an [*acl.Declarative]; pass
	// [AllowAllCopyVisibility] to opt out explicitly. See [requireCopyGates]
	// for why the tolerance is conditioned rather than unconditional.
	CopyVisibility CopyReader

	// CopyGuard evaluates a copy definition's `guard:` permission against the
	// SOURCE entity, so "the owner of THIS doc may publish it" works without
	// a global grant. Nil makes a GUARDED copy fail closed, matching the
	// statemachine's nil-guard rule; an unguarded copy is unaffected.
	CopyGuard CopyGuard

	// CopyReadGate answers "may this principal READ the copy's source at
	// all", before either half of the elevation split runs (TKT-C1XUA8).
	// Elevation decides which FIELDS travel, not whether the principal may
	// touch the entity.
	//
	// Nil: accepted without a policy — a deployment with no acl.yaml has no
	// gate to consult, and every other read on it is ungated too. REJECTED
	// when ACL is an [*acl.Declarative]; pass [AllowAllCopyReadGate] to opt
	// out explicitly. This used to say "required in spirit and nil-tolerant
	// in practice", which is exactly the gap [requireCopyGates] closes.
	CopyReadGate CopyReadGate

	// AttachmentLocker serializes changes to one (entity, file property)
	// between the attachment service, the copy engine and DeleteEntityFace;
	// see [AttachmentLocker].
	//
	// Nil: accepted only when the metamodel declares no `file` property,
	// because then there are no references to count. Rejected otherwise:
	// without the lock a copy racing a delete could leave a reference to
	// deleted bytes.
	AttachmentLocker AttachmentLocker
}

// FieldWriteGate answers whether the ctx principal may write the named
// properties of e. Defined here at the call site (CLAUDE.md consumer-side
// interfaces) so entitymanager needs no dependency on the affordance
// resolver that backs it in production.
//
// It runs on PatchEntity and CreateEntity of a [FieldGated] handle, after
// the row-level decision (RR-32XA5V). UpdateEntity and RecreateEntity never
// run it: their callers check fields first or act for the operator.
// Production wires the policy-backed gate from internal/affordances, or
// [AllowAllFieldGate] for a deployment without field grants (TKT-0XL8MF).
//
// set holds the properties being upserted (key → new value); unset holds
// the ones being removed. An implementation returns a non-nil error to
// refuse the write; the error surfaces to the caller unwrapped, so it
// should carry whatever structured denial detail the surface needs.
//
// **Scope: caller-authored changes only.** Automation-derived properties
// ([automation.Result.PropertiesSet]) are system writes and are
// deliberately NOT passed through this gate. The gate enforces parity with
// what the affordance resolver would surface to this principal on a read —
// and automation is the system acting, not the principal. Gating it would
// mean a user who cannot author `status` could never trigger an automation
// that sets `status`, breaking ordinary workflow automations (RR-00ERM9).
type FieldWriteGate interface {
	CheckFieldWrite(ctx context.Context, e *entity.Entity, set map[string]any, unset []string) error
}

// AllowAllFieldGate permits every field write. It is the explicit opt-out
// for surfaces that sit on the operator trust boundary (the CLI, where the
// caller already has full filesystem access to the data) and for tests.
//
// Named and passed deliberately, exactly like [acl.NopACL] — never the
// result of leaving [Deps.FieldGate] nil.
type AllowAllFieldGate struct{}

// CheckFieldWrite implements [FieldWriteGate] by allowing everything.
func (AllowAllFieldGate) CheckFieldWrite(
	context.Context, *entity.Entity, map[string]any, []string,
) error {
	return nil
}

// TransitionEnforcer is the narrow contract the Manager needs from the
// compiled state machines: enforce an update (old→new) and a create's entry
// value. Defined at the call site (CLAUDE.md consumer-side interfaces);
// [*statemachine.Set] satisfies it.
// CopyReader is the caller-scoped read a CROSS-ENTITY copy sees. Narrow by
// design: the copy needs one lookup, and taking the whole visibility.Reader
// would bind this package to methods it never calls.
//
// Get returns (nil, false, nil) indistinguishably for denied, missing and
// type-mismatched — whether an entity exists is a genuine secret.
type CopyReader interface {
	// Get returns the entity's stored face as the ctx principal may see it:
	// row-gated, FACE-gated (a `type@face` grant that excludes face is a
	// miss), and field-redacted. Denied and absent are indistinguishable.
	Get(ctx context.Context, entityType, id string, face entity.Face) (*entity.Entity, bool, error)
}

// CopyGuard answers a copy definition's `guard:` permission for one subject.
//
// The same shape as statemachine.Guard, and satisfied by the same wiring —
// the point of reusing the vocabulary is that the two cannot drift into
// asking different questions. Subject is the bare entity id, matching the
// conferral model: identity-scoped roles confer on the entity in every
// world, and the face-granularity already lives in the write grant.
// CopyReadGate is the row-level read verdict for a copy's SOURCE. Narrow by
// design; satisfied by the same acl.Request the rest of the read path uses.
type CopyReadGate interface {
	// PermitsReadFace is the row verdict for the id AND the face allowlist
	// for the stored face being read. Face-blind gating here would let a
	// principal granted `read: [page@published]` promote a draft it may not
	// read; [acl.Request.PermitsReadFace] is the production implementation.
	PermitsReadFace(ctx context.Context, entityType, entityID string, face entity.Face) (bool, error)
}

// AllowAllCopyReadGate permits reading every copy source. It is the explicit
// opt-out from [requireCopyGates] for a surface that sits on the operator
// trust boundary, and for tests that build a policy-backed [Deps] without
// caring about the copy path.
//
// Named and passed deliberately, exactly like [AllowAllFieldGate] and
// [acl.NopACL] — never the result of leaving [Deps.CopyReadGate] nil, because
// that is the shape a forgotten wiring takes.
type AllowAllCopyReadGate struct{}

// PermitsReadFace implements [CopyReadGate] by permitting everything.
func (AllowAllCopyReadGate) PermitsReadFace(
	context.Context, string, string, entity.Face,
) (bool, error) {
	return true, nil
}

// AllowAllCopyVisibility reads a copy source raw — no row gate, no field
// redaction. The explicit opt-out counterpart to [AllowAllCopyReadGate];
// the same "named, never nil" rule applies.
//
// Unlike the other opt-outs it needs a store to read through, so the field is
// unexported and [NewAllowAllCopyVisibility] is the only way to build one. A
// bare `AllowAllCopyVisibility{}` would pass [requireCopyGates] and then nil-
// panic on the first cross-entity copy — the deferred-downstream-symptom
// failure this whole guard exists to prevent, reintroduced by its own opt-out.
type AllowAllCopyVisibility struct{ store store.Store }

// NewAllowAllCopyVisibility builds the ungated copy reader.
//
// Nil: st is rejected — a nil store here is broken under ANY ACL, not only a
// policy-backed one, which is why this is validated at construction rather
// than inside [requireCopyGates].
func NewAllowAllCopyVisibility(st store.Store) (AllowAllCopyVisibility, error) {
	if st == nil {
		return AllowAllCopyVisibility{}, errors.New(
			"entitymanager: NewAllowAllCopyVisibility: Store is required")
	}
	return AllowAllCopyVisibility{store: st}, nil
}

// Get implements [CopyReader] by reading the stored face ungated. entityType
// is ignored: allow-all draws no per-type distinction.
//
// A store error is reported as a MISS rather than propagated, matching
// appbuild's real copyVisibility so a caller's `!ok` handling is uniform
// across the two. Note the usual "absent and denied are indistinguishable"
// rationale does NOT apply here — this gate denies nothing, so there is no
// confidentiality property to preserve; the reason is shape-consistency
// alone. Both share the same wart: a transient store failure surfaces to the
// operator as ErrCopySourceMissing for an entity that plainly exists.
func (v AllowAllCopyVisibility) Get(
	ctx context.Context, _, id string, face entity.Face,
) (*entity.Entity, bool, error) {
	e, err := v.store.GetEntity(ctx, entity.Ref{ID: id, Face: face})
	if err != nil {
		return nil, false, nil //nolint:nilerr // a miss, to match copyVisibility's shape; see the doc comment
	}
	return e, true, nil
}

type CopyGuard interface {
	HoldsPermission(ctx context.Context, entityID, permission string) bool
}

type TransitionEnforcer interface {
	EnforceUpdate(
		ctx context.Context, old, updated *entity.Entity,
		guard statemachine.Guard, lookup statemachine.GraphLookup,
	) error
	EnforceCreate(ctx context.Context, e *entity.Entity) error
}

// New constructs a Manager and validates required collaborators.
func New(d Deps) (*Manager, error) {
	if d.Store == nil {
		return nil, errors.New("entitymanager: New: Store is required")
	}
	if d.Meta == nil {
		return nil, errors.New("entitymanager: New: Meta is required")
	}
	if d.Templater == nil {
		return nil, errors.New("entitymanager: New: Templater is required")
	}
	if d.Audit == nil {
		return nil, errors.New("entitymanager: New: Audit is required (use audit.Nop{} to opt out)")
	}
	if d.ACL == nil {
		return nil, errors.New("entitymanager: New: ACL is required (use acl.NopACL{} to opt out)")
	}
	if d.Transitions == nil {
		return nil, errors.New(
			"entitymanager: New: Transitions is required (use statemachine.Compile; an empty set is a no-op)")
	}
	if d.Computed == nil {
		compiled, err := computed.Compile(d.Meta)
		if err != nil {
			return nil, fmt.Errorf("entitymanager: New: %w", err)
		}
		d.Computed = compiled
	}
	if d.FieldGate == nil {
		return nil, errors.New(
			"entitymanager: New: FieldGate is required (use entitymanager.AllowAllFieldGate{} to opt out)")
	}
	if (d.Automations == nil) != (d.Cascade == nil) {
		return nil, errors.New(
			"entitymanager: New: Automations and Cascade must be supplied together (both non-nil or both nil)",
		)
	}
	if err := requireCopyGates(d); err != nil {
		return nil, err
	}
	if d.AttachmentLocker == nil && metamodel.HasFileProperties(d.Meta) {
		return nil, errors.New(
			"entitymanager: New: AttachmentLocker is required when the metamodel declares a file property")
	}
	return &Manager{deps: d}, nil
}

// requireCopyGates refuses a policy-backed deployment that forgot to wire the
// copy read gates.
//
// [Deps.CopyReadGate] and [Deps.CopyVisibility] are nil-tolerant on purpose:
// a deployment with no acl.yaml has no gate to consult, and every other read
// on it is raw too. But that tolerance is only safe while nil actually MEANS
// "no policy". When a real policy is present and these two are nil, the copy
// path degrades silently: [copyEngine.authorizeCopy] skips its read check
// entirely and [copyEngine.readCopySource] takes the raw-store branch for
// cross-entity copies, so a principal can read a source entity — and the
// `visible:`-hidden fields on it — that every other read path would refuse.
// There is no error and no log line; the only marker was a code comment.
//
// So the nil-tolerance is conditioned rather than removed. The test for "a
// real policy" is that the ACL is an [*acl.Declarative]: that is the only
// implementation compiled from an operator's acl.yaml, and it is the identical
// test appbuild's buildEntityManager already applies to decide whether the
// gates it wires behave actively. [acl.NopACL] and [acl.ReadOnlyACL] are
// deliberately excluded — the former IS the no-policy case, and the latter
// denies every write without expressing any per-principal read policy for a
// gate to enforce.
//
// Checking d.ACL == nil instead would be vacuous: ACL is already required
// above, so it is never nil by the time this runs.
//
// The assertion is safe only because the implementation set is CLOSED — an
// unrecognized type falls through to "no policy", i.e. open, whereas
// internal/dataentry spells the same question as a switch that fails closed.
// A decorating implementation would split those two apart. That is guarded in
// the acl package itself by TestACLImplementations_AreAClosedSet, which fails
// on a new implementation and names the sites to update.
//
// This is the same fail-fast posture as [Deps.FieldGate] (RR-X9NVHI:
// forgotten wiring must not become an ACL bypass) and the sibling
// [Deps.CopyGuard], which already fails closed on a guarded copy. Those two
// made the copy READ gates the odd ones out; this closes that asymmetry at
// the load-time moment the rest of New already uses.
//
// Nil: rejected when ACL is *acl.Declarative — pass AllowAllCopyReadGate and
// AllowAllCopyVisibility to opt out explicitly.
func requireCopyGates(d Deps) error {
	if _, policyActive := d.ACL.(*acl.Declarative); !policyActive {
		return nil
	}
	var missing, optOuts []string
	if d.CopyReadGate == nil {
		missing = append(missing, "CopyReadGate")
		optOuts = append(optOuts, "AllowAllCopyReadGate{}")
	}
	if d.CopyVisibility == nil {
		missing = append(missing, "CopyVisibility")
		optOuts = append(optOuts, "NewAllowAllCopyVisibility(store)")
	}
	if len(missing) == 0 {
		return nil
	}
	// Name only what is actually missing, in both halves of the message: a
	// half-wired Deps that is told to re-wire the gate it already supplied
	// sends the operator looking at correct code.
	return fmt.Errorf(
		"entitymanager: New: %s required when ACL is a compiled policy "+
			"(*acl.Declarative): a nil copy read gate makes the copy path read its source "+
			"ungated and unredacted. Wire the real gate, or pass %s to opt out explicitly",
		strings.Join(missing, " and "), strings.Join(optOuts, " / "))
}

// authorizeAndAudit consults the ACL and, on deny, records a
// `denied-write` audit row and returns [*acl.ForbiddenError]. On allow,
// returns nil and the caller proceeds. Called as the first
// non-validation step in every write entry point.
//
// The denied-write audit happens regardless of audit backend
// (Filesystem / Memory / Nop) — forensic posture demands recording
// what was attempted, not just what landed.
//
// When this Manager is an elevated handle (`m.bypassACL` — TKT-D8T148: a
// `rela.bypass_acl(...)` write), the ACL deny is SKIPPED: the write is allowed
// regardless of the principal's grants. Elevation is a property of WHICH
// Manager you hold, not of the context — see Manager.elevated. The real
// principal is still preserved (`principal.From(ctx)`, unchanged) and the
// bypass is recorded (recordACLBypass) so the audit trail is unambiguous about
// which writes were elevated and on whose behalf. We do NOT recordDeniedWrite
// on the bypass path — there is no denial to record.
func (m *Manager) authorizeAndAudit(ctx context.Context, req acl.WriteRequest) error {
	if m.cascadeWrite {
		return nil
	}
	if m.bypassACL {
		if !isAffordanceProbe(ctx) {
			m.recordACLBypass(ctx, req)
		}
		return nil
	}
	decision := m.deps.ACL.AuthorizeWrite(ctx, req)
	if decision.Allow {
		return nil
	}
	if !isAffordanceProbe(ctx) {
		// An affordance PROBE is not an attempted write, so recording it would
		// make the audit log say something untrue. See [withAffordanceProbe].
		m.recordDeniedWrite(ctx, decision, req)
	}
	return &acl.ForbiddenError{Decision: decision}
}

// affordanceProbeKey marks a context as an AFFORDANCE QUERY: a read-only
// "could this principal do X" question, not an attempted write.
type affordanceProbeKey struct{}

// withAffordanceProbe marks ctx as an affordance query, suppressing audit
// records from the authorization path.
//
// # Why the audit log must not see these
//
// [CopiesForSource] answers "which copies may this principal invoke here" by
// running the REAL authorization path — that is what stops the hint drifting
// from the write. But that path audits its denials, and a denial recorded for
// a question nobody asked makes `op=denied-write` mean "someone looked at a
// page" rather than "someone tried to write and was refused".
//
// A SPA renders an entity view, the view lists copy affordances, and every
// page load appends N rows to an append-only log. Anyone alerting on
// denied-write volume gets paged by ordinary browsing, and the real signal
// drowns. The audit log's whole value is that it does not lie (see
// [Manager.CopyState]'s note on why audit lands after the commit); this keeps
// that true in the other direction.
//
// It suppresses ONLY the record, never the decision: the verdict is computed
// identically and returned identically, so the hint still cannot drift.
//
// The key is typed and unexported, so nothing outside this package can mark a
// real write as a probe.
func withAffordanceProbe(ctx context.Context) context.Context {
	return context.WithValue(ctx, affordanceProbeKey{}, true)
}

// isAffordanceProbe reports whether ctx was marked by [withAffordanceProbe].
func isAffordanceProbe(ctx context.Context) bool {
	v, _ := ctx.Value(affordanceProbeKey{}).(bool)
	return v
}

// mapTransitionError translates a state-machine enforcement error into the
// entitymanager's wire-facing error shape (TKT-E4LW2). A guard denial becomes
// an [*acl.ForbiddenError] (RuleKind "transition-guard") so it flows through
// the same 403 path — and audit row — as any other authorization denial;
// legality and precondition failures pass through unchanged and surface as 422
// validation-class errors at the HTTP boundary. Returns nil for a nil input.
func (m *Manager) mapTransitionError(ctx context.Context, subject acl.Subject, err error) error {
	if err == nil {
		return nil
	}
	var ge *statemachine.GuardError
	if errors.As(err, &ge) {
		decision := acl.Decision{
			Allow:    false,
			RuleKind: "transition-guard",
			RuleID:   ge.Permission, // the specific right, queryable in audit (RR-F30CZ/N1)
			Reason:   err.Error(),
		}
		m.recordDeniedWrite(ctx, decision, acl.WriteRequest{Op: acl.OpUpdate, Subject: subject})
		return &acl.ForbiddenError{Decision: decision}
	}
	return err
}

// recordACLBypass emits one audit row for an elevated (rela.bypass_acl) write.
// It preserves the real triggering principal (so "who really caused this" is
// always answerable, like ruid under sudo) and marks the row acl_bypass=true
// so forensic queries can isolate elevated writes. The op stays the genuine
// write op; the bypass marker rides the Summary alongside the existing
// triggered_by=automation:<name>.
func (m *Manager) recordACLBypass(ctx context.Context, req acl.WriteRequest) {
	var subject *audit.Subject
	switch s := req.Subject.(type) {
	case acl.RelationSubject:
		subject = &audit.Subject{Kind: "relation", RelationType: s.Type, FromID: s.FromID}
	case acl.EntitySubject:
		subject = &audit.Subject{Kind: "entity", Type: s.Type(), ID: s.ID()}
	}
	m.deps.Audit.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpACLBypass,
		Subject:     subject,
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     fmt.Sprintf("acl_bypass=true op=%s", req.Op),
	})
}

// recordDeniedWrite emits one audit row describing the refused
// attempt. Subject names the would-be target (entity or relation);
// Summary carries the deny rule_kind / rule_id / reason and the
// attempted op so jq filters can ask "what did Alice try to do?".
func (m *Manager) recordDeniedWrite(ctx context.Context, d acl.Decision, req acl.WriteRequest) {
	// RR-79HD: surface the target ID (entity ID, or relation
	// from-ID) so forensic queries against the audit log can answer
	// "which specific entity did Alice try to mutate?" without
	// re-parsing the deny summary string. ToID is omitted because
	// RR-F9M9 removed it from RelationSubject.
	var subject *audit.Subject
	switch s := req.Subject.(type) {
	case acl.RelationSubject:
		subject = &audit.Subject{
			Kind:         "relation",
			RelationType: s.Type,
			FromID:       s.FromID,
		}
	case acl.EntitySubject:
		subject = &audit.Subject{
			Kind: "entity",
			Type: s.Type(),
			ID:   s.ID(),
		}
	}
	m.deps.Audit.Record(audit.Record{
		Time:        time.Now().UTC(),
		Op:          audit.OpDeniedWrite,
		Subject:     subject,
		Principal:   principal.From(ctx),
		TriggeredBy: audit.TriggeredByFrom(ctx),
		Summary:     formatDeniedSummary(d, req.Op),
	})
}

// formatDeniedSummary builds the audit Summary for a denied-write row.
// Appends `attribution=[role=X via source, ...]` when the Decision
// carries Attributions so operators can answer "which roles did the
// resolver consider and via which paths" without re-running the
// resolver (AC7). The wire 403 path stays opaque — only audit reads
// Attributions.
func formatDeniedSummary(d acl.Decision, op acl.Op) string {
	base := fmt.Sprintf("denied: %s (rule_kind=%s rule_id=%s) attempted op=%s",
		d.Reason, d.RuleKind, d.RuleID, op)
	if len(d.Attributions) == 0 {
		return base
	}
	parts := make([]string, 0, len(d.Attributions))
	for _, a := range d.Attributions {
		parts = append(parts, fmt.Sprintf("role=%s via %s", a.Role, a.Source.String()))
	}
	return base + " attribution=[" + strings.Join(parts, ", ") + "]"
}

// CreateEntity creates a new entity, runs on-create automations, and
// dispatches any resulting cascade.
//
// Pipeline:
//
//  1. createCore: ID generation, template application, defaults,
//     metamodel validation, persist to store.
//  2. If automation should run: engine.Process(EventEntityCreated) →
//     collect property changes → apply → re-persist (yes, two writes:
//     the first is the validated bare entity, the second carries any
//     automation-set properties; pinned by manager_test.go).
//  3. Dispatch cascade via Cascade.Process; merge outcome into the
//     entity.CreateResult.
//
// **Caller-entity mutation.** The supplied `*entity.Entity` is used as
// a property/content carrier and not retained — the freshly-built
// entity is returned via [entity.CreateResult.Entity]. Callers should consume
// the returned entity, not the one they passed in.
func (m *Manager) CreateEntity(
	ctx context.Context, e *entity.Entity, opts entity.CreateOptions,
) (*entity.CreateResult, error) {
	ctx = withStoreAttribution(ctx)
	if e == nil {
		return nil, errors.New("entitymanager: CreateEntity: entity is nil")
	}
	// The face is taken from opts, NOT from e — the same value createCore
	// writes below. Reading it off the caller's carrier entity is what made
	// the authorized face and the written face two different things
	// (BUG-HC6I2T); e.Face was structurally always zero, so the check
	// answered about a row the write never touched.
	if err := m.deps.requireCreateFaceFor(e.Type, opts.Face); err != nil {
		return nil, err
	}
	if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      acl.OpCreate,
		Subject: acl.NewEntitySubject(e.Type, opts.ID, opts.Face),
	}); err != nil {
		return nil, err
	}
	// Field-level gate on the caller's properties only, after the row-level
	// decision. Template defaults and automation output are applied later
	// and are not the caller's writes (RR-00ERM9). Without this, a field the
	// caller may not write could be set at create time (BUG-Q60V).
	if fieldGated(m) {
		candidate := &entity.Entity{ID: opts.ID, Type: e.Type, Face: opts.Face, Properties: e.Properties}
		if err := checkFieldWrite(ctx, m, candidate, e.Properties, nil); err != nil {
			return nil, err
		}
	}
	if err := rejectFileCreate(m.deps.Meta, e.Type, e.Properties); err != nil {
		return nil, err
	}
	if opts.ID != "" {
		if def, ok := m.deps.Meta.GetEntityDef(e.Type); ok && !def.IsManualID() {
			return nil, customIDNotAllowedError(e.Type, def, opts.ID)
		}
		// No GetEntity pre-check: it was a TOCTOU duplicate of the
		// store's atomic uniqueness guarantee. createCore now writes
		// with a direct CreateEntity and surfaces a conflict as
		// ErrEntityAlreadyExists, so a racing create can't slip past a
		// passed pre-check and become an overwrite.
	}

	created, warnings, err := createCore(ctx, m.deps, e.Type, createCoreOpts{
		ID:              opts.ID,
		IDPrefix:        opts.Prefix,
		TemplateVariant: opts.Variant,
		Properties:      e.Properties,
		Content:         e.Content,
		Face:            opts.Face,
	})
	if err != nil {
		return nil, err
	}
	// State-machine entry is enforced INSIDE createCore, before the durable
	// write (RR-HETEE), so an illegal entry never persists. No guard applies on
	// create-entry today; if a future change adds one, route its ErrGuardDenied
	// through mapTransitionError so it surfaces as 403, not 422 (RR-F30CZ/N2).

	result := &entity.CreateResult{Entity: created, Warnings: warnings}

	// Audit the durable write now, before automation re-writes or the
	// cascade run. createCore has already persisted the entity; if a
	// later step fails (the post-automation upsert, or a cascade that
	// hard-errors) the entity is still on disk, so the audit log must
	// already reflect it. Recording after the cascade left a window
	// where a committed write produced no audit record.
	m.recordEntityAudit(ctx, audit.OpCreateEntity, created, "created")

	runAutomation := m.deps.Automations != nil && !opts.SkipAutomation
	if !runAutomation {
		return result, nil
	}

	autoResult := m.deps.Automations.Process(ctx, automation.Event{
		Type:   automation.EventEntityCreated,
		Entity: created,
	})
	if len(autoResult.PropertiesSet) > 0 {
		// A compare-and-swap onto the stored row, not a write of `created`:
		// a concurrent patch that landed after createCore must survive. The
		// unique constraint is re-enforced against the POST-automation values
		// inside the same write, so an automation that sets a `unique`
		// property cannot write a duplicate natural key (the create path must
		// not be the weaker one). It is an update, never an upsert: createCore
		// already persisted the row (BUG-ZWTDH9).
		written, err := writeAutomationProperties(ctx, m.deps, created, autoResult.PropertiesSet, nil)
		if err != nil {
			return nil, err
		}
		created = written
		result.Entity = created
		// Recompute warnings against the post-automation state
		// (DEC-HWZHA). The pre-write warnings from createCore reflect
		// the entity before automation set any properties.
		if errs := m.deps.Meta.ValidateEntity(created.ID, created.Type, created.Properties); len(errs) > 0 {
			_, result.Warnings = partitionValidationErrors(errs)
		}
	}
	result.AutomationWarnings = autoResult.Warnings
	result.AutomationErrors = autoResult.Errors

	outcome, cascadeErr := m.deps.Cascade.Process(ctx, &cascadeHost{deps: m.deps}, autocascade.Request{
		Trigger:    created,
		OldTrigger: nil,
		Result:     autoResult,
		Scripts:    m.deps.ScriptRunner,
		Mutator:    m.gated(), // gated() so elevation never propagates into a nested cascade (TKT-D8T148)
	})
	if cascadeErr != nil {
		return nil, fmt.Errorf("cascade: %w", cascadeErr)
	}
	result.RelationsCreated = outcome.RelationsCreated
	result.EntitiesCreated = outcome.EntitiesCreated
	result.AutomationErrors = append(result.AutomationErrors, outcome.Errors...)
	result.AutomationWarnings = append(result.AutomationWarnings, outcome.Warnings...)

	return result, nil
}

// ValidateCreate runs the create path's defaults + validation against a
// candidate entity WITHOUT persisting, authorizing, auditing, or
// running automation. It returns the would-be entity (post template /
// status defaults) and the DEC-HWZHA soft warnings the real create
// would surface — so a dry-run create can show as-you-type validation
// feedback that cannot drift from [Manager.CreateEntity] (both share
// [buildCandidateEntity]).
//
// Contract:
//   - No write, no audit row, no automation — it is advisory only.
//     The real CreateEntity remains the sole authorization and audit
//     point; callers MUST re-authorize at commit (e.g. the data-entry
//     create handler's affordance gate).
//   - Hard structural errors (unknown type, bad manual ID, ID-prefix
//     mismatch) return as an error; soft conditions (required-unset,
//     type / value mismatch) return as warnings on a nil error.
//   - opts.ID may be empty: an ID is generated only to satisfy
//     validation that doesn't depend on it; it is not reserved.
func (m *Manager) ValidateCreate(
	ctx context.Context, e *entity.Entity, opts entity.CreateOptions,
) (*entity.Entity, []entity.Warning, error) {
	if e == nil {
		return nil, nil, errors.New("entitymanager: ValidateCreate: entity is nil")
	}
	// The face check runs here too, or the dry-run would report a create
	// clean that the real one refuses — the drift this function exists to
	// prevent (it shares buildCandidateEntity for exactly that reason).
	if err := m.deps.requireCreateFaceFor(e.Type, opts.Face); err != nil {
		return nil, nil, err
	}
	if err := rejectFileCreate(m.deps.Meta, e.Type, e.Properties); err != nil {
		return nil, nil, err
	}
	return buildCandidateEntity(ctx, m.deps, e.Type, createCoreOpts{
		ID:              opts.ID,
		IDPrefix:        opts.Prefix,
		TemplateVariant: opts.Variant,
		Properties:      e.Properties,
		Content:         e.Content,
		Face:            opts.Face,
		// Skip the full-store scan generateID would do — dry-run runs
		// per debounced keystroke and a real ID is not needed for
		// validation. RR-8I07.
		SkipIDGeneration: true,
	})
}

// UpdateEntity validates the new state, runs on-update automation
// when an old state is available, applies property changes, persists,
// and dispatches the cascade.
//
// **Caller-entity mutation.** When automation sets properties via
// [automation.Result.PropertiesSet], UpdateEntity mutates the supplied
// `*entity.Entity` in place before writing. Callers that need to
// preserve the pre-call state should clone first.
//
// **Gate:** if the entity doesn't exist, UpdateEntity returns
// [ErrEntityNotFound] and never runs the engine. (Preserves
// pre-refactor workspace behavior.)
func (m *Manager) UpdateEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error) {
	ctx = withStoreAttribution(ctx)
	if e == nil {
		return nil, errors.New("entitymanager: UpdateEntity: entity is nil")
	}
	// Face comes from the entity being written, because that is what the
	// store keys on (stateKey(e.ID, e.Face)). Omitting it would authorize
	// every faced write against the DEFAULT face — the zero Face means
	// "default state" — so a role holding only a bare `update: [policy]`
	// grant could write `policy@published`, inverting the invariant
	// GrantsVerbOnState exists to hold (BUG-Y0GNSB).
	if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      acl.OpUpdate,
		Subject: acl.NewEntitySubject(e.Type, e.ID, e.Face),
	}); err != nil {
		return nil, err
	}
	// Hard-validate BEFORE the existence probe. Order is load-bearing and
	// predates the updateCore extraction: an invalid entity reports its
	// validation error even when the id does not exist. PatchEntity cannot
	// share this half (it has no candidate entity until after the read), so
	// the pre-check lives here rather than in updateCore, which re-runs the
	// same partition on the merged result.
	preErrs := m.deps.Meta.ValidateEntity(e.ID, e.Type, e.Properties)
	if hard, _ := partitionValidationErrors(preErrs); len(hard) > 0 {
		return nil, newValidationError(hard)
	}

	// The pre-image is read at the face this write AUTHORIZED against, not
	// at the zero coordinate. The old spelling read GetEntity(id), the zero
	// face, so it decided against e.Face and then read a different row
	// — the same authorize-here/write-there split BUG-HC6I2T removed from
	// the create path, and on a faced type it simply never found anything.
	oldEntity, getErr := m.deps.Store.GetEntity(ctx, entity.Ref{ID: e.ID, Face: e.Face})
	if getErr != nil {
		// Fails closed: only a genuine miss is reported as missing, so a
		// transient store error cannot reach the not-found branch that
		// returns before authorizing.
		if !errors.Is(getErr, store.ErrNotFound) {
			return nil, getErr
		}
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, e.ID)
	}
	if err := rejectComputedChanges(m.deps, oldEntity, e); err != nil {
		return nil, err
	}
	if err := rejectFileChanges(m.deps.Meta, oldEntity, e); err != nil {
		return nil, err
	}
	if len(metamodel.FileProperties(m.deps.Meta, e.Type)) > 0 {
		return updateKeepingFileValues(ctx, m, e, oldEntity)
	}

	// Unconditional: UpdateEntity is the whole-entity save, whose caller owns
	// every field. A caller wanting compare-and-swap uses PatchEntity with
	// entity.Patch.ExpectedVersion.
	return m.updateCore(ctx, e, oldEntity, "")
}

// updateKeepingFileValues is the write half of [Manager.UpdateEntity] for a
// type with file properties. The caller's file values have passed the
// file-property rule against old; they are replaced by the stored values
// (see [pinStoredFileValues]) and the write is conditional on old's version.
// A stamp that lands between the read and the write is therefore retried
// against the new row rather than reverted, which would leave the face
// naming bytes the attachment service has already deleted. The rest of the
// save stays last-writer-wins: each retry writes the caller's other values
// again.
func updateKeepingFileValues(
	ctx context.Context, m *Manager, e, old *entity.Entity,
) (*entity.UpdateResult, error) {
	first := true
	return patchWithRetry(ctx, true, func(bool) (*entity.UpdateResult, error) {
		if !first {
			fresh, err := m.deps.Store.GetEntity(ctx, entity.Ref{ID: e.ID, Face: e.Face})
			if err != nil {
				if errors.Is(err, store.ErrNotFound) {
					return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, e.ID)
				}
				return nil, err
			}
			if err := rejectComputedChanges(m.deps, fresh, e); err != nil {
				return nil, err
			}
			old = fresh
		}
		first = false
		pinStoredFileValues(m.deps.Meta, old, e, "")
		return m.updateCore(ctx, e, old, string(store.VersionOf(old)))
	})
}

// PatchEntity applies a TARGETED set of property changes to one entity:
// properties the patch does not name are preserved as-is, regardless of
// whether the caller could read them. See [entity.Patch] for the
// merge semantics (upserts, then unsets, then the body tri-state).
//
// This is the safe alternative to read-modify-write. A caller doing
// GetEntity → merge → UpdateEntity must hold the WHOLE entity, so any
// property it failed to carry across is destroyed on save — and a caller
// reading through a redacting reader cannot carry across what it cannot
// see. PatchEntity owns the read internally and merges against the RAW
// stored entity, so that failure mode is unreachable: forgetting a
// property is a no-op, not an erasure.
//
// **Ordering is load-bearing** (RR-32XA5V):
//
//	read → locked-check → authorize → field gate → merge → updateCore
//
// The read comes first because the ACL subject needs the entity's real
// type and the caller supplied only an id — the same shape, and the same
// accepted existence disclosure, as [Manager.DeleteEntity]. The field gate
// runs strictly AFTER authorization: field verdicts are value-dependent,
// so consulting them for an unauthorized caller would turn allow-vs-deny
// into an oracle for stored property values and for entity existence.
//
// An elevated Manager (rela.bypass_acl) skips the field gate as well as
// the row ACL. Elevation is total by design — a half-elevated handle that
// silently drops some property writes is the confusing contract
// lua.WriteDeps.ElevatedManager exists to avoid — and the bypass is still
// recorded by authorizeAndAudit (RR-BA1NIV).
//
// # Compare-and-swap (TKT-34XS2R)
//
// Setting [entity.Patch.ExpectedVersion] makes the write conditional: it
// applies only if the stored entity still matches that token, else it fails
// with a [store.VersionConflictError] (recoverable via errors.As, and
// errors.Is-able as [store.ErrConflict]). The precondition is evaluated by
// the STORE, atomically with the write — this method's own read is write-prep
// for the merge, not the check, so no TOCTOU window exists between them.
//
// A caller that wants a bounded retry loop re-reads, re-derives its patch,
// and retries with the conflict's Actual version. ExpectedVersion is for a
// patch derived from something the caller read — notably a Content
// replacement computed from a base read.
//
// Without ExpectedVersion the patch is STILL written as a compare-and-swap,
// against the row this method read as its merge base, and retried a bounded
// number of times (read, authorize, merge, write) when another writer lands
// in between. The merge writes the whole row, so an unconditional write
// would erase a concurrent patch to a DIFFERENT property. Nothing serializes
// writers above the store, in this process or across processes, so the store
// precondition is what keeps disjoint patches from losing each other.
// Automations only compute a result before the write, and the cascade runs
// after it, so a retried attempt has no side effects to repeat.
func (m *Manager) PatchEntity(
	ctx context.Context, id string, p entity.Patch,
) (*entity.UpdateResult, error) {
	ctx = withStoreAttribution(ctx)
	if id == "" {
		return nil, errors.New("entitymanager: PatchEntity: id is empty")
	}
	return patchWithRetry(ctx, p.ExpectedVersion == "", func(pinToRead bool) (*entity.UpdateResult, error) {
		return m.patchEntityOnce(ctx, id, p, pinToRead, "")
	})
}

// patchWithRetry runs one patch attempt, and with pinToRead retries it a
// bounded number of times when another writer lands between its read and
// its write. A free function so [Attachments.StampAttachments] shares the
// loop without adding a Manager method.
func patchWithRetry(
	ctx context.Context, pinToRead bool, once func(pinToRead bool) (*entity.UpdateResult, error),
) (*entity.UpdateResult, error) {
	for attempt := 1; ; attempt++ {
		res, err := once(pinToRead)
		if pinToRead && isVersionConflict(err) && attempt < casRetryAttempts {
			if berr := casBackoff(ctx, attempt); berr != nil {
				return nil, berr
			}
			continue
		}
		return res, err
	}
}

// patchEntityOnce is one attempt of [Manager.PatchEntity]. With pinToRead the
// write is conditional on the version of the row it read, rather than on
// p.ExpectedVersion. fileProp names the one file property the write may
// change; only [Attachments.StampAttachments] passes one.
func (m *Manager) patchEntityOnce(
	ctx context.Context, id string, p entity.Patch, pinToRead bool, fileProp string,
) (*entity.UpdateResult, error) {
	// RAW read, deliberately ungated: this is write-prep, and the merge
	// base must be the complete stored entity or hidden properties would
	// be dropped from the clone and erased on save. Consolidating this
	// read here is the point of the primitive — consumers no longer hold
	// a raw store handle of their own.
	//
	// The id may be the fused boundary form ("POL-1@published"), so it is
	// PARSED rather than used as a bare id: a bare id addresses the zero
	// face, and a type declaring faces stores no row at the zero
	// coordinate, so the faced form would resolve nothing (BUG-HC6I2T). The authorization below already reads the face
	// off the stored row, so resolving it here is what makes that correct
	// rather than accidentally right for unfaced types only.
	stored, getErr := m.getEntityByRef(ctx, id)
	if getErr != nil {
		// Structural, not textual: consumers holding a narrow write
		// interface (the Lua bindings) must be able to tell this apart
		// from other hard errors without matching on the message.
		return nil, newEntityNotFound(id)
	}

	// A locked (git-crypt) entity reads as a shell whose real property
	// values are unavailable, so merging onto it and saving would persist
	// the shell OVER the encrypted content — the same erasure this
	// primitive exists to prevent, via encryption instead of redaction
	// (RR-0QWLRC). Matches RecreateEntity's guard.
	if stored.IsLocked() {
		return nil, fmt.Errorf("entitymanager: PatchEntity: entity %s has inaccessible fields", id)
	}

	// Face from the STORED entity, not from `id`: `id` may be the fused
	// boundary form ("POL-1@published"), which stateKey resolves onto the
	// faced row — so the face actually being written is stored.Face, and
	// authorizing without it would decide against the default face
	// (BUG-Y0GNSB).
	if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op: acl.OpUpdate,
		// stored.ID, not `id`: the caller may have passed the fused form
		// ("POL-4@draft"), and the subject names the id and the face in
		// SEPARATE fields — passing the fused string as the ID would make the
		// row-gate key disagree with every other write path, which names the
		// bare id (see UpdateEntity above).
		Subject: acl.NewEntitySubject(stored.Type, stored.ID, stored.Face),
	}); err != nil {
		return nil, err
	}

	// Field-level gate, after the row-level decision.
	if fieldGated(m) {
		if err := checkFieldWrite(ctx, m, stored, p.Properties, p.MetaUnset); err != nil {
			return nil, err
		}
	}
	if err := rejectComputedPatch(m.deps, stored.Type, p.Properties, p.MetaUnset); err != nil {
		return nil, err
	}
	if err := rejectFilePatch(m.deps.Meta, stored, p, fileProp); err != nil {
		return nil, err
	}

	updated := stored.Clone()
	p.Apply(updated)
	pinStoredFileValues(m.deps.Meta, stored, updated, fileProp)

	expected := p.ExpectedVersion
	if pinToRead {
		expected = string(store.VersionOf(stored))
	}
	return m.updateCore(ctx, updated, stored, expected)
}

// updateCore is the shared post-authorization update pipeline: validate,
// run on-update automation, enforce transitions and unique constraints,
// persist, audit, and dispatch the cascade.
//
// Method on Manager (not a free function over [Deps] like [createCore])
// because the cascade needs m.gated() to stop elevation propagating into
// descendants. It deliberately contains **no ACL check and no attribution**
// — both belong to the entry points ([Manager.UpdateEntity], [Manager.PatchEntity]), which
// authorize with the subject shape appropriate to how they learned the
// entity type. Putting authorize here would double-authorize PatchEntity
// (which must authorize early, before it can merge) and emit two
// denied-write audit rows for one denial.
//
// oldEntity is passed in rather than re-read: both callers already hold it
// (UpdateEntity to prove existence, PatchEntity as the merge base), so the
// manager itself reads the row once per write. Callers may still read
// separately for their own reasons — internal/mcp does, to validate
// property names against the entity type before dispatching.
// expectedVersion carries the caller's compare-and-swap precondition down to
// the durable write. Empty means unconditional; PatchEntity passes the
// version of its merge base even when its caller gave none.
func (m *Manager) updateCore(
	ctx context.Context, e, oldEntity *entity.Entity, expectedVersion string,
) (*entity.UpdateResult, error) {
	// Type is immutable on update, on EVERY path:
	// the store checks a non-default face's type against its family but not
	// the bare row's, so a retype here would split the family — the bare row
	// one type, its sibling faces another, and on fsstore two files under two
	// type directories (see ErrTypeImmutable).
	if oldEntity != nil && e.Type != oldEntity.Type {
		return nil, fmt.Errorf("entitymanager: %s: %w (stored %q, body %q)",
			e.ID, ErrTypeImmutable, oldEntity.Type, e.Type)
	}
	if err := m.deps.Computed.Evaluate(ctx, e); err != nil {
		return nil, err
	}
	// DEC-HWZHA: partition validation errors once. Hard errors abort;
	// soft conditions populate Result.Warnings. If automation runs and
	// mutates properties, we recompute warnings against the post-
	// automation state.
	preErrs := m.deps.Meta.ValidateEntity(e.ID, e.Type, e.Properties)
	hard, soft := partitionValidationErrors(preErrs)
	if len(hard) > 0 {
		return nil, newValidationError(hard)
	}

	result := &entity.UpdateResult{Entity: e, Warnings: soft}

	autoResult, ranAutomation, err := m.processUpdateAutomation(ctx, e, oldEntity)
	if err != nil {
		return nil, err
	}
	if ranAutomation {
		if len(autoResult.PropertiesSet) > 0 {
			// Properties changed — recompute warnings against the
			// post-automation state (DEC-HWZHA).
			if errs := m.deps.Meta.ValidateEntity(e.ID, e.Type, e.Properties); len(errs) > 0 {
				_, result.Warnings = partitionValidationErrors(errs)
			} else {
				result.Warnings = nil
			}
		}
		result.AutomationWarnings = autoResult.Warnings
		result.AutomationErrors = autoResult.Errors
	}

	// Enforce enum state machines on the final (post-automation) state, using
	// the prior state to determine the transition (TKT-E4LW2). This is the
	// unforgettable chokepoint: the enforcer is a required collaborator run in
	// the fixed write pipeline, so no update path can skip legality/guard/
	// precondition. An empty enforcer (metamodel with no transitions) is a
	// no-op.
	if err := m.deps.Transitions.EnforceUpdate(
		ctx, oldEntity, e, m.deps.TransitionGuard, m.deps.TransitionGraph,
	); err != nil {
		return nil, m.mapTransitionError(ctx, acl.NewEntitySubject(e.Type, e.ID, e.Face), err)
	}

	// Enforce `unique: true` natural-key constraints against the final
	// (post-automation) property values, atomically with the write,
	// excluding this entity's own prior version so a re-save of an unchanged
	// value does not collide.
	//
	// UpdateEntity, not upsert: the GetEntity above already established
	// the row exists (else we returned ErrEntityNotFound), so this is
	// unambiguously an update (BUG-ZWTDH9).
	// The CAS precondition rides down to the store, which is the only layer
	// that can compare-and-write atomically. Empty expectedVersion yields the
	// zero condition: an unconditional write, for a caller that owns the
	// whole entity (UpdateEntity).
	if err := writeWithUniqueCheck(ctx, m.deps, e, e.ID, func(st store.Store) error {
		_, werr := st.UpdateEntityIf(ctx, e, store.UpdateCondition{
			ExpectedVersion: store.EntityVersion(expectedVersion),
		})
		return werr
	}); err != nil {
		var invalid *ValidationError
		if errors.As(err, &invalid) {
			return nil, err
		}
		// A derived unique-property index can reject an update whose (possibly
		// automation-set) value duplicates another entity's, even though the
		// scan above passed under a concurrent writer. Surface it as the same
		// 422 the scan produces (TKT-3Q0GP1); other errors pass through.
		if ok, mapped := mapUniquePropertyConflict(err); ok {
			return nil, mapped
		}
		// %w, and nothing more: a *store.VersionConflictError MUST stay
		// recoverable by errors.As all the way up to whatever retry loop the
		// caller wrote. RR-HI9QIU is the cautionary case — a translation that
		// dropped the cause made a retry loop unreachable and turned 8 of 8
		// concurrent losers into 500s, silently.
		return nil, fmt.Errorf("write entity: %w", err)
	}

	// Audit the durable write now, before the cascade run. The entity
	// is already persisted; gating the audit on cascade success left a
	// window where a committed write produced no audit record.
	m.recordEntityAudit(ctx, audit.OpUpdateEntity, e, updateEntitySummary(oldEntity, e))

	if !ranAutomation {
		return result, nil
	}

	outcome, cascadeErr := m.deps.Cascade.Process(ctx, &cascadeHost{deps: m.deps}, autocascade.Request{
		Trigger:    e,
		OldTrigger: oldEntity,
		Result:     autoResult,
		Scripts:    m.deps.ScriptRunner,
		Mutator:    m.gated(), // gated() so elevation never propagates into a nested cascade (TKT-D8T148)
	})
	if cascadeErr != nil {
		return nil, fmt.Errorf("cascade: %w", cascadeErr)
	}
	result.RelationsCreated = outcome.RelationsCreated
	result.EntitiesCreated = outcome.EntitiesCreated
	result.AutomationErrors = append(result.AutomationErrors, outcome.Errors...)
	result.AutomationWarnings = append(result.AutomationWarnings, outcome.Warnings...)

	return result, nil
}

func (m *Manager) processUpdateAutomation(
	ctx context.Context, e, oldEntity *entity.Entity,
) (*automation.Result, bool, error) {
	if m.deps.Automations == nil {
		return nil, false, nil
	}
	result := m.deps.Automations.Process(ctx, automation.Event{
		Type:      automation.EventEntityUpdated,
		Entity:    e,
		OldEntity: oldEntity,
	})
	if len(result.PropertiesSet) == 0 {
		return result, true, nil
	}
	if err := rejectComputedPresent(m.deps, e.Type, stringMapAny(result.PropertiesSet)); err != nil {
		return nil, true, err
	}
	for prop, val := range result.PropertiesSet {
		e.SetString(prop, val)
	}
	if err := m.deps.Computed.Evaluate(ctx, e); err != nil {
		return nil, true, err
	}
	return result, true, nil
}

// authorizeCascadeRelations checks the principal may delete every relation
// type a cascade will destroy, and returns the first denial.
//
// Deduplicated by (relation type, source entity type): the decision is a pure
// function of those two plus the op, so a hub entity with thousands of edges
// across a handful of types costs a handful of checks. Without the dedup a
// denied cascade on a 5,000-edge entity would also write 5,000 denied-write
// audit rows for ONE refused operation, burying the signal it exists to give.
//
// Note the asymmetry with the live write path: DeleteRelation authorizes
// against the SOURCE entity's type, so an incoming edge is checked against its
// own source, not against the entity being deleted. That is the same subject
// the principal would face deleting the edge directly, which is the property
// that makes this gate meaningful rather than merely stricter.
func (m *Manager) authorizeCascadeRelations(
	ctx context.Context, tx store.Store, id string, self []*entity.Entity, incoming, outgoing []*entity.Relation,
) error {
	// The FACE is part of the key, not just the pair (BUG-64MU2Q): a
	// content-scoped edge is authorized against the state that owns it, so
	// collapsing draft- and published-tailed edges into one check would let
	// a `policy@published` grant stand for a draft-tailed edge. The
	// cardinality stays bounded by (types × faces), which is small.
	//
	// familyFaces joins the source family's faces, because an identity edge
	// from a faced source is authorized on each of them (D4): two sources of
	// one type that store different faces are different decisions.
	type subject struct {
		relType, fromType string
		fromFace          entity.Face
		familyFaces       string
	}
	// The decision is cached, not just marked seen: a later edge of the same
	// subject must get the same answer, or the hidden-edge rule below would
	// depend on which of two equal edges came first.
	decided := make(map[subject]error)
	// One lookup per source id, not per edge: every face of a family has the
	// same type, so a hub's thousands of edges from a few sources cost a few
	// reads.
	familyOf := make(map[string]entityFamily)

	check := func(rel *entity.Relation) error {
		if rel == nil {
			return nil
		}
		// Resolve through the TX view, not the outer store. The whole point of
		// running in a transaction is that the authorized set and the deleted
		// set are derived under one serialization; reading the type that the
		// decision is MADE ON from outside it would undercut exactly that. On
		// pgstore the outer handle is the pool, so this would also take a
		// second connection while the first is held.
		//
		// An unresolvable source yields an empty FromType, which fails
		// closed. A store error aborts the delete, unlike the live relation
		// writes, which still proceed with an empty type: both refuse, but
		// only this one reports the real cause.
		fam, known := familyOf[rel.From]
		if !known {
			var err error
			fam, err = lookupFamily(ctx, tx, rel.From)
			if err != nil && !errors.Is(err, store.ErrNotFound) {
				return fmt.Errorf("resolve source of %s --%s--> %s: %w",
					entity.FormatStateRef(rel.From, rel.FromFace), rel.Type, rel.To, err)
			}
			familyOf[rel.From] = fam
		}
		sub := relationWriteSubject(m.deps.Meta, rel.Type, fam, "", rel.FromFace)
		key := subject{
			relType: rel.Type, fromType: sub.FromType, fromFace: rel.FromFace,
			familyFaces: fmt.Sprint(sub.FamilyFaces),
		}
		if err, done := decided[key]; done {
			return err
		}

		// FromID is deliberately EMPTY. The decision is a pure function of
		// (relation type, source type, source face, op) — FromID is never
		// read by any branch of authorizeRelationWrite — so one check stands
		// for every edge sharing that tuple. Stamping one arbitrary id into
		// the audit row would make it look like a claim about that specific
		// entity: a forensic query for another source in the same class
		// would find nothing, though it was equally refused. An empty
		// FromID says "this type-and-face class", which is what was
		// actually decided.
		err := m.authorizeAndAudit(ctx, acl.WriteRequest{Op: acl.OpDelete, Subject: sub})
		decided[key] = err
		return err
	}

	// A relation the caller cannot see is nonexistent to them, so a denial
	// over one names nothing about it (BUG-1BXQDD). It is reported only when
	// no visible relation is denied: answering with the first denial in store
	// order would tell a caller who expects a visible edge's denial that a
	// hidden edge came first. Every edge counts as hidden when visibility
	// cannot be decided.
	vis := newEdgeVisibility(faceGateOf(m), tx)
	visErr := vis.seed(ctx, self)
	if visErr == nil {
		visErr = vis.load(ctx, append(append([]*entity.Relation(nil), incoming...), outgoing...))
	}
	var hiddenErr error
	judge := func(rel *entity.Relation, err error, describe func() error) error {
		if err == nil {
			return nil
		}
		if visErr == nil && vis.visible(rel) {
			return describe()
		}
		if hiddenErr == nil {
			hiddenErr = hiddenEdgeFailure(id, err)
		}
		return nil
	}

	// Split by direction so the error can name the FAR endpoint. For an
	// incoming edge the entity being deleted IS rel.To, so "its X relation to
	// <To>" would say "its relation to itself" and withhold the one fact that
	// makes the error actionable: the other endpoint, whose type is what
	// actually blocked the delete.
	for _, rel := range incoming {
		err := check(rel)
		if jErr := judge(rel, err, func() error {
			return fmt.Errorf("cannot delete %s: its incoming %s relation from %s: %w",
				id, rel.Type, entity.FormatStateRef(rel.From, rel.FromFace), err)
		}); jErr != nil {
			return jErr
		}
	}
	for _, rel := range outgoing {
		err := check(rel)
		if jErr := judge(rel, err, func() error {
			return fmt.Errorf("cannot delete %s: its outgoing %s relation to %s: %w",
				id, rel.Type, rel.To, err)
		}); jErr != nil {
			return jErr
		}
	}
	if visErr != nil && hiddenErr != nil {
		slog.Error("entitymanager: cannot tell which relations the caller sees on a delete",
			"id", id, "error", visErr)
	}
	return hiddenErr
}

// cascadeCapture is what deleteEntityInTx hands back for the caller to act on
// AFTER the transaction closes: the incident relations the cascade destroyed,
// captured before the delete removed them.
type cascadeCapture struct {
	incoming []*entity.Relation
	outgoing []*entity.Relation
	// owned lists the owned entities deleted with this one; their relations
	// are recorded with them, not in incoming/outgoing.
	owned []ownedDeletion
}

// deleteEntityInTx is DeleteEntity's critical section: collect the incident
// relations, authorize every one a cascade would destroy, then delete.
//
// It performs STORE WORK ONLY. Version capture, alias notification and audit
// all happen in the caller, after the transaction closes — see the Tx comment
// at the call site for why that separation is load-bearing rather than
// stylistic.
func (m *Manager) deleteEntityInTx(
	ctx context.Context, tx store.Store, id string, cascade bool, authorized familyAuthorization,
) (*store.DeleteResult, *cascadeCapture, error) {
	// Re-read the family under the transaction and authorize any face that
	// appeared since the caller's check: the store deletes the family it
	// re-derives here, not the one the caller saw (BUG-1YN750).
	family, fErr := familyRows(ctx, tx, id)
	if fErr != nil {
		return nil, nil, fmt.Errorf("list faces of %q: %w", id, fErr)
	}
	if aErr := m.authorizeFamily(ctx, acl.OpDelete, id, family, authorized); aErr != nil {
		return nil, nil, aErr
	}

	incoming, cErr := collectIncidentRelations(ctx, tx, id, store.DirectionIncoming)
	if cErr != nil {
		return nil, nil, fmt.Errorf("collect incoming relations for %q: %w", id, cErr)
	}
	outgoing, cErr := collectIncidentRelations(ctx, tx, id, store.DirectionOutgoing)
	if cErr != nil {
		return nil, nil, fmt.Errorf("collect outgoing relations for %q: %w", id, cErr)
	}
	totalRelations := len(incoming) + len(outgoing)

	if totalRelations > 0 && !cascade {
		return nil, nil, ErrHasRelations
	}

	// A cascade destroys these edges, so the principal must be allowed to
	// delete each one — otherwise deleting an entity is a back door to
	// removing relation types you hold no delete grant on.
	//
	// This MUST run inside the transaction: both backends re-derive the
	// incident set under their own lock and delete THAT set, so authorizing
	// a set collected outside the serialization would leave a window for a
	// concurrent writer. Denials abort before any write, so nothing unwinds
	// — which matters because fs/mem do not roll back (store.Transactor).
	if cascade && totalRelations > 0 {
		if aErr := m.authorizeCascadeRelations(ctx, tx, id, nil, incoming, outgoing); aErr != nil {
			return nil, nil, aErr
		}
	}

	// Entities the owner owns go with it (TKT-QO14GB). All of them are
	// authorized here, before the first write, for the reason the edge check
	// above gives.
	capture := &cascadeCapture{incoming: incoming, outgoing: outgoing}
	if cascade {
		owned, oErr := prepareOwnedDeletes(ctx, m, tx, id, outgoing)
		if oErr != nil {
			return nil, nil, oErr
		}
		capture.owned = owned
		if dErr := deleteOwned(ctx, m, tx, id, owned); dErr != nil {
			return nil, capture, dErr
		}
		withoutRelationsOf(capture, owned)
	}

	// Delegate the actual deletion to the store's cascade, which removes
	// the relation files and the entity file under a single lock and aborts
	// fail-secure if any relation file cannot be removed — so the entity is
	// never deleted while a relation is left behind (issue #888).
	res, delErr := tx.DeleteFamily(ctx, id, cascade)
	if delErr != nil {
		// Propagate res AND the capture, not nil: a non-transactional backend
		// reports the relations it DID remove before aborting, and the caller
		// needs both to record them — audit AND version history (issue #929).
		// Capturing only one leaves the two logs contradicting each other.
		// res is nil on a transactional backend, which the caller handles.
		return res, capture, fmt.Errorf("delete entity: %w", delErr)
	}
	// The store's own scan is the final word on what went. A face it removed
	// that the re-read above did not see is authorized now. On pg/sqlite a
	// denial rolls the delete back. On fs the watcher indexes an external
	// file edit without taking the Tx lock, so a face can land between the
	// re-read and the delete, and fs cannot roll back: res is returned with
	// the error so the caller can record what really went (issue #929).
	if aErr := m.authorizeFamily(ctx, acl.OpDelete, id, res.DeletedEntities, authorized); aErr != nil {
		return res, capture, aErr
	}
	return res, capture, nil
}

// DeleteEntity removes an entity and its incident relations.
// **No automation, no cascade.** When cascade is false and the
// entity has any incident relations, returns [ErrHasRelations]
// without deleting anything.
func (m *Manager) DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error) {
	// Every face, not one: a delete addresses the whole FAMILY, so it is
	// authorized on each face it removes (BUG-1YN750). Authorizing the single row
	// a one-face read returned let a principal holding delete on `policy@draft` only
	// remove `policy@published` as well.
	//
	// Fails closed on a non-not-found error: collapsing every failure into
	// ErrEntityNotFound would let a transient store error skip the ACL check
	// below, since that branch returns before authorizing (existence is
	// itself a secret). Only a genuine miss is reported as missing.
	family, err := familyRows(ctx, m.deps.Store, id)
	if err != nil {
		return nil, err
	}
	if len(family) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}
	// ACL check happens after the lookup so the request carries the
	// real entity type; a deny on a non-existent entity would be more
	// confusing than the ErrEntityNotFound returned above.
	authorized := make(familyAuthorization, len(family))
	if aclErr := m.authorizeFamily(ctx, acl.OpDelete, id, family, authorized); aclErr != nil {
		return nil, aclErr
	}

	// Everything from here to the store delete runs inside ONE Tx.
	//
	// The incident-relation set must be collected, authorized and deleted
	// under a single serialization: both stores RE-DERIVE the set inside
	// their own lock/tx (fsstore rebuilds from its live index, pgstore
	// re-reads inside the transaction) and delete THAT set — not the one
	// handed to them. Authorizing a snapshot taken outside the lock would
	// leave a window in which a concurrent writer adds an edge that is then
	// deleted with no authorization at all, which is exactly the "back door
	// to destroying edge types you cannot delete directly" this gate exists
	// to close. The family's faces are re-authorized there for the same
	// reason.
	//
	// Reads inside the callback go through the OUTER handle (that is what
	// acl.StoreGraph holds). fsstore's readers never take txMu, so this does
	// not deadlock — pinned by TestTx_ReadsViaOuterHandleDoNotDeadlock.
	// Writes must use the tx view, per the Tx contract.
	//
	// Everything EXTERNAL stays out of the callback: version capture, alias
	// notification and audit all run below, after the transaction closes.
	// store.Transactor is explicit that slow external I/O inside fn makes the
	// whole deployment's writers wait (pgstore holds a global advisory lock
	// for the callback's duration), and the version recorder writes through
	// its own pool connection — so a capture emitted inside would commit even
	// when the delete rolls back, leaving history asserting a delete that
	// never happened.
	var (
		res      *store.DeleteResult
		captured *cascadeCapture
	)
	txErr := m.deps.Store.Tx(ctx, func(tx store.Store) error {
		txCtx := store.ContextInTx(ctx)
		var dErr error
		res, captured, dErr = m.deleteEntityInTx(txCtx, tx, id, cascade, authorized)
		return dErr
	})
	if txErr != nil {
		// A non-transactional backend can fail partway through a cascade with
		// some relation files already off disk, and it reports those in a
		// partial result (issue #929). Audit them before propagating: the
		// deletion really happened, and a log that omits it is a log that
		// denies the system's actual state.
		//
		// Same label and same emitter as the success path below, so a partial
		// and a complete cascade are indistinguishable in the log except by
		// how many rows they produced. A delete-entity record is written only
		// for a face the store reports removing and that is really gone; a
		// face that survived gets none.
		m.recordPartialCascade(ctx, "cascade:delete-entity:"+id, res, captured)
		if captured != nil {
			ownedTB := ownerDeleteTrigger(id)
			for _, o := range captured.owned {
				m.recordPartialCascade(audit.WithTriggeredBy(ctx, ownedTB), ownedTB, o.res, o.capture)
			}
		}
		return nil, txErr
	}

	// Version capture, after commit. The rows are written from the pre-delete
	// state captured inside the transaction, so their content is the same as
	// an in-transaction capture would have produced; only the timing differs.
	// A crash between commit and here loses the delete markers — the same
	// non-atomicity the pre-Tx code carried and documented, not a regression.
	//
	// One capture per face the store reports removing: each face is its own
	// version lineage (entity_versions keys on face), so capturing one face
	// left every other face's history without a delete marker.
	for _, e := range res.DeletedEntities {
		m.recordEntityVersion(ctx, store.VersionOpDelete, e, "")
	}

	// Tell id-keyed subscribers the entity is gone. They decide what to do:
	// the CalDAV alias service RETAINS its reference, as the tombstone that
	// stops a stale client write resurrecting this entity.
	m.notifyAliasesOfDelete(ctx, id)

	// Capture a final version for every relation this cascade destroyed. This
	// is the ONLY place cascade-deleted relations get versioned: the store's
	// DeleteEntity bulk-deletes them below the write choke-point, so without
	// this their history would silently end with no `delete` marker and no
	// restore path (RR-181AFY).
	if captured != nil {
		cascadeTB := "cascade:delete-entity:" + id
		for _, rel := range captured.incoming {
			m.recordRelationVersion(ctx, store.VersionOpDelete, rel, "", "", cascadeTB)
		}
		for _, rel := range captured.outgoing {
			m.recordRelationVersion(ctx, store.VersionOpDelete, rel, "", "", cascadeTB)
		}
	}

	// Audit exactly what the store reports deleting. Cascade-deleted
	// relations carry triggered_by so the log attributes them to this
	// delete; recordRelationAudit reads it from cascadeCtx.
	cascadeCtx := ctx
	if cascade && len(res.DeletedRelations) > 0 {
		cascadeCtx = audit.WithTriggeredBy(ctx, "cascade:delete-entity:"+id)
	}
	for _, rel := range res.DeletedRelations {
		m.recordRelationAudit(cascadeCtx, audit.OpDeleteRelation, rel, "deleted")
	}

	cascaded := 0
	if cascade {
		cascaded = len(res.DeletedRelations)
	}
	m.recordFamilyDeleteAudit(ctx, res.DeletedEntities, cascaded)

	out := &entity.DeleteResult{
		DeletedEntities:  res.DeletedEntities,
		DeletedRelations: res.DeletedRelations,
	}
	if captured != nil {
		recordOwnedDeletes(ctx, m, id, captured.owned)
		for _, o := range captured.owned {
			out.DeletedEntities = append(out.DeletedEntities, o.res.DeletedEntities...)
			out.DeletedRelations = append(out.DeletedRelations, o.res.DeletedRelations...)
		}
	}
	return out, nil
}

// familyRows returns every stored face of the entity family id, in the
// store's stable order. An empty result means the family does not exist.
//
// IDs-scoped, never a full scan, like lookupFamily.
func familyRows(ctx context.Context, st store.Store, id string) ([]*entity.Entity, error) {
	return store.Family(ctx, st, id)
}

// familyAuthorization is the set of (type, face) subjects one family-wide
// operation (delete or rename) has already authorized. The type is part of
// the key because it is part of the ACL subject: a state stored under a
// different type is a different subject and is authorized on its own.
type familyAuthorization map[familyFace]bool

type familyFace struct {
	typ  string
	face entity.Face
	// family marks the one family-level subject a faced type is renamed
	// under ([acl.NewFamilySubject]); face is then zero.
	family bool
}

// authorizeRename authorizes the rename of the family id, adding each
// allowed subject to authorized.
//
// A row of a faced type is authorized ONCE per type, at family level
// ([acl.NewFamilySubject]): the rename moves every face, hidden ones
// included, so the decision comes from the principal's `rename:` grants and
// never from which faces are stored. Authorizing each stored face made the
// refusal depend on faces the caller cannot read, which disclosed that a
// hidden face exists. A faceless row keeps its per-row check, since its one
// row is the whole family.
func (m *Manager) authorizeRename(
	ctx context.Context, id string, family []*entity.Entity, authorized familyAuthorization,
) error {
	for _, e := range family {
		if len(declaredFaces(m.deps.Meta, e.Type)) == 0 {
			if err := m.authorizeFamily(ctx, acl.OpRename, id, []*entity.Entity{e}, authorized); err != nil {
				return err
			}
			continue
		}
		key := familyFace{typ: e.Type, family: true}
		if authorized[key] {
			continue
		}
		if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
			Op:      acl.OpRename,
			Subject: acl.NewFamilySubject(e.Type, id),
		}); err != nil {
			return err
		}
		authorized[key] = true
	}
	return nil
}

// faceReadGate is the read verdict a whole-family write asks so that its
// answer depends only on rows the caller may read. [*acl.Declarative]
// implements it; an ACL that does not (NopACL, ReadOnlyACL, test fakes)
// expresses no read policy, so every row counts as readable. It is an
// optional capability of [Deps.ACL], like the store's, rather than a Deps
// field: the read policy and the write policy are the same compiled object.
type faceReadGate interface {
	PermitsReadFace(ctx context.Context, entityType, entityID string, face entity.Face) (bool, error)
}

// faceGateOf returns the read gate m's whole-family writes consult, or nil
// when every row counts as readable: an ACL without a read policy, or
// bypass_acl and automation cascades, which skip authorization altogether
// (see Manager.authorizeAndAudit).
func faceGateOf(m *Manager) faceReadGate {
	gate, ok := m.deps.ACL.(faceReadGate)
	if !ok || m.bypassACL || m.cascadeWrite {
		return nil
	}
	return gate
}

// readableRows returns the rows of family gate admits, in order; all of them
// for a nil gate. A gate error is returned: the caller must not decide on a
// guess.
func readableRows(ctx context.Context, gate faceReadGate, family []*entity.Entity) ([]*entity.Entity, error) {
	if gate == nil {
		return family, nil
	}
	out := make([]*entity.Entity, 0, len(family))
	for _, e := range family {
		readable, err := gate.PermitsReadFace(ctx, e.Type, e.ID, e.Face)
		if err != nil {
			return nil, fmt.Errorf("read gate for %s: %w", entity.FormatStateRef(e.ID, e.Face), err)
		}
		if readable {
			out = append(out, e)
		}
	}
	return out, nil
}

// requireReadableRow reports [ErrEntityNotFound] when family holds rows but
// gate admits none of them, so a hidden entity is refused exactly like an
// absent one. An empty family passes: the caller's own not-found path
// reports it.
func requireReadableRow(ctx context.Context, gate faceReadGate, id string, family []*entity.Entity) error {
	if len(family) == 0 {
		return nil
	}
	readable, err := readableRows(ctx, gate, family)
	if err != nil {
		return err
	}
	if len(readable) == 0 {
		return fmt.Errorf("%w: %s", ErrEntityNotFound, id)
	}
	return nil
}

// authorizeFamily authorizes op on every row of family whose subject is not
// yet in authorized, and adds each allowed subject to it. It stops at the
// first denial, so a denied family operation leaves exactly one denied-write
// record, as a single-face denial does.
//
// The subject names id, the family the caller addressed, rather than each
// row's own ID: a rename re-reads the family under its new id and still
// authorizes the rename of the old one.
//
// Under bypass_acl each face is a separate bypassed write, so each gets its
// own acl-bypass record. When this runs inside the Tx a denial writes its
// denied-write record there, as authorizeCascadeRelations does.
func (m *Manager) authorizeFamily(
	ctx context.Context, op acl.Op, id string, family []*entity.Entity, authorized familyAuthorization,
) error {
	for _, e := range family {
		key := familyFace{typ: e.Type, face: e.Face}
		if authorized[key] {
			continue
		}
		if err := m.authorizeAndAudit(ctx, acl.WriteRequest{
			Op:      op,
			Subject: acl.NewEntitySubject(e.Type, id, e.Face),
		}); err != nil {
			return err
		}
		authorized[key] = true
	}
	return nil
}

// recordFamilyDeleteAudit writes one delete-entity record per removed face.
// The audit subject has no face field, so a faced row names its face in the
// summary, as DeleteEntityFace does. The cascade count describes the family
// delete as a whole, so it goes on the first record only; repeating it would
// read as that many relations per face.
func (m *Manager) recordFamilyDeleteAudit(ctx context.Context, deleted []*entity.Entity, cascaded int) {
	for i, e := range deleted {
		summary := "deleted"
		if !e.Face.IsImplicit() {
			summary = "deleted face " + string(e.Face)
		}
		if i == 0 && cascaded > 0 {
			summary = fmt.Sprintf("%s (cascade: %d relations)", summary, cascaded)
		}
		m.recordEntityAudit(ctx, audit.OpDeleteEntity, e, summary)
	}
}

// facesStillStored reports whether any row in deleted is still in the store,
// which after a failed Tx means the backend rolled the delete back. A read
// error counts as still stored: recording a delete that did not happen is
// the worse error for a log whose value is that it does not lie.
func facesStillStored(ctx context.Context, st store.Store, deleted []*entity.Entity) bool {
	for _, e := range deleted {
		_, err := st.GetEntity(ctx, entity.Ref{ID: e.ID, Face: e.Face})
		if err == nil {
			return true
		}
		if !errors.Is(err, store.ErrNotFound) {
			slog.Error("entitymanager: cannot tell whether a failed delete removed a face",
				"id", e.ID, "face", e.Face, "error", err)
			return true
		}
	}
	return false
}

// recordPartialCascade records the relations and faces a FAILED delete had
// already removed from disk, so BOTH logs reflect what genuinely happened
// rather than nothing at all (TKT-A23L87 / issue #929).
//
// Audit and version history together, deliberately. RR-181AFY made
// DeleteResult.DeletedRelations the single source for ALL relation-delete
// capture precisely so the two cannot drift; recording only the audit half
// here would leave the log asserting a deletion that history denies, and the
// rows are already off disk so no sweep could backfill them.
//
// Driven by res.DeletedRelations and res.DeletedEntities, not by the captured
// incident set: only the rows the store actually removed may be recorded. The capture supplies
// the pre-delete snapshots those ids need.
//
// cascadeTB is the triggered_by label on the relation records, the one the
// success path uses for the same rows: "cascade:delete-entity:<id>" for the
// deleted entity's own relations, [ownerDeleteTrigger] for an owned entity's.
//
// Nil-safe throughout: a transactional backend returns nil on error, and a
// failure on the first relation removes nothing. Either way this is a no-op.
func (m *Manager) recordPartialCascade(
	ctx context.Context, cascadeTB string, res *store.DeleteResult, captured *cascadeCapture,
) {
	if res == nil {
		return
	}
	// A face-authorization denial after the store delete returns the full
	// result on every backend. pg/sqlite then roll back, so nothing may be
	// recorded; fs cannot, so what it removed is recorded as removed.
	if facesStillStored(ctx, m.deps.Store, res.DeletedEntities) {
		return
	}
	for _, e := range res.DeletedEntities {
		m.recordEntityVersion(ctx, store.VersionOpDelete, e, "")
	}
	m.recordFamilyDeleteAudit(ctx, res.DeletedEntities, len(res.DeletedRelations))
	if len(res.DeletedRelations) == 0 {
		return
	}
	cascadeCtx := audit.WithTriggeredBy(ctx, cascadeTB)

	// Index the captured pre-delete snapshots so each removed relation is
	// versioned from the state it actually had, not from a reconstruction.
	snapshots := make(map[string]*entity.Relation)
	if captured != nil {
		for _, rel := range append(append([]*entity.Relation{}, captured.incoming...), captured.outgoing...) {
			snapshots[relationKey(rel)] = rel
		}
	}

	for _, rel := range res.DeletedRelations {
		m.recordRelationAudit(cascadeCtx, audit.OpDeleteRelation, rel, "deleted")

		snap := rel
		if s, ok := snapshots[relationKey(rel)]; ok {
			snap = s
		}
		m.recordRelationVersion(ctx, store.VersionOpDelete, snap, "", "", cascadeTB)
	}
}

// relationKey is the (from, type, to) identity of a relation, used to pair a
// store-reported deletion with its pre-delete snapshot.
func relationKey(r *entity.Relation) string {
	return r.From + "--" + r.Type + "--" + r.To
}

// readFaceToDelete reads the face a delete removes, mapping a missing row to
// [ErrEntityNotFound].
func readFaceToDelete(ctx context.Context, st store.Store, id string, face entity.Face) (*entity.Entity, error) {
	e, err := st.GetEntity(ctx, entity.Ref{ID: id, Face: face})
	if errors.Is(err, store.ErrNotFound) {
		return nil, fmt.Errorf("%w: %s", ErrEntityNotFound, entity.FormatStateRef(id, face))
	}
	return e, err
}

// DeleteEntityFace removes ONE non-bare content state of an entity and the
// content-scoped edges that belong to it, leaving the rest of the family
// standing — what a DELETE addressed to `ID@face` means, and the only way to
// "unpublish" short of deleting the entity (TKT-SLFURL).
//
// It is [Manager.DeleteEntity]'s sibling, not a narrower spelling of it:
// DeleteEntity sweeps the whole family and every incident edge on both sides,
// whereas a face owns only the OUTGOING edges tailed at that face — incoming
// edges point at the entity, not at one of its states, and survive while any
// face remains (see [store.EntityWriter.DeleteFace] for the rule). Deleting
// the family's LAST face removes the entity, so every incident edge goes with
// it and each is authorized like a DeleteEntity cascade (RR-2466U1). The
// cascade flag means what it means for DeleteEntity, and applies only to the
// last face: without it, a last face with edges is [ErrHasRelations]. A face
// that is not the last always takes its own edges.
//
// "Last" is judged on the faces the CALLER may read, for every check: the
// cascade opt-in and the edges authorized. A face whose siblings are all
// hidden is treated as last even though the store keeps the entity, so the
// response is the same whether or not a hidden face exists (see
// faceDeleteEdges). A face the caller cannot read is [ErrEntityNotFound]. The bare face is refused
// here rather than delegated, because "delete the bare face" is either the
// whole entity (when no other face exists) or undefined (when one does), and
// neither is what a caller who spelled a face meant.
//
// Authorization names the face, so a role holding the bare `delete: [policy]`
// cannot remove a published face — the same narrowing every state-shaped
// write grant applies (GrantsVerbOnState). The cascade edges are authorized
// inside the transaction for the reason DeleteEntity gives: the store
// re-derives the set under its own lock and deletes THAT set.
func (m *Manager) DeleteEntityFace(
	ctx context.Context, id string, face entity.Face, cascade bool,
) (*entity.DeleteResult, error) {
	if face.IsImplicit() {
		return nil, fmt.Errorf("delete face: %s names the bare face; delete the entity instead", id)
	}
	// Fails closed, as in DeleteEntity: the ACL check is below.
	current, err := readFaceToDelete(ctx, m.deps.Store, id, face)
	if err != nil {
		return nil, err
	}
	// A face the caller cannot read is refused like an absent one, before the
	// ACL check, so a 403 never confirms it exists.
	gate := faceGateOf(m)
	if rErr := requireReadableRow(ctx, gate, id, []*entity.Entity{current}); rErr != nil {
		return nil, rErr
	}
	if aclErr := m.authorizeAndAudit(ctx, acl.WriteRequest{
		Op:      acl.OpDelete,
		Subject: acl.NewEntitySubject(current.Type, id, current.Face),
	}); aclErr != nil {
		return nil, aclErr
	}
	// Read before the Tx: the gate may query through the outer store, which on
	// pgstore is a second connection. A sibling that appears in between
	// counts as unreadable, which only makes the checks stricter.
	readableSiblings, err := readableSiblingFaces(ctx, gate, m.deps.Store, id, face)
	if err != nil {
		return nil, err
	}
	var (
		res                *store.DeleteResult
		lastFace           bool
		incoming, outgoing []*entity.Relation
	)
	txErr := m.deps.Store.Tx(ctx, func(tx store.Store) error {
		txCtx := store.ContextInTx(ctx)
		// Re-read the face under the transaction (BUG-J3PBFN). The read above
		// only answers not-found and denial early; the version and audit
		// record must carry the row this transaction deletes, not one a
		// concurrent update has since replaced. A row whose type changed in
		// between is a different ACL subject and is authorized again.
		inTx, rErr := readFaceToDelete(txCtx, tx, id, face)
		if rErr != nil {
			return rErr
		}
		if inTx.Type != current.Type {
			if aErr := m.authorizeAndAudit(txCtx, acl.WriteRequest{
				Op:      acl.OpDelete,
				Subject: acl.NewEntitySubject(inTx.Type, id, inTx.Face),
			}); aErr != nil {
				return aErr
			}
		}
		current = inTx

		var (
			callerLast bool
			cErr       error
		)
		incoming, outgoing, callerLast, lastFace, cErr = faceDeleteEdges(txCtx, tx, id, face, readableSiblings)
		if cErr != nil {
			return cErr
		}
		// The last face removes the entity, so it takes the same opt-in a
		// family delete does before cutting edges (RR-2466U1). "Last" is the
		// caller's view, so a hidden sibling cannot change the answer.
		if callerLast && !cascade && len(incoming)+len(outgoing) > 0 {
			return ErrHasRelations
		}
		if len(incoming)+len(outgoing) > 0 {
			if aErr := m.authorizeCascadeRelations(txCtx, tx, id, nil, incoming, outgoing); aErr != nil {
				return aErr
			}
		}
		var dErr error
		res, dErr = tx.DeleteFace(txCtx, entity.Ref{ID: id, Face: face})
		if dErr != nil {
			return fmt.Errorf("delete face: %w", dErr)
		}
		// The store decides "last face" again under its own lock. Every edge
		// it removed must be one authorized above; otherwise fail, so a
		// transactional backend rolls the delete back.
		return requireAuthorizedEdges(res.DeletedRelations, incoming, outgoing)
	})
	if txErr != nil {
		// A non-transactional backend can fail part-way with some relation
		// files already gone (TKT-A23L87); record those, as DeleteEntity does.
		m.recordPartialCascade(ctx, "cascade:delete-entity:"+id, res, &cascadeCapture{incoming: incoming, outgoing: outgoing})
		return nil, txErr
	}

	// Version capture, audit and relation attribution after commit, exactly
	// as DeleteEntity orders them and for the same reasons.
	m.recordEntityVersion(ctx, store.VersionOpDelete, current, "")
	notifyAliasesOfFaceDelete(ctx, m.deps.AliasRewriter, id, face)
	if lastFace {
		// The entity is gone, not just a face of it.
		m.notifyAliasesOfDelete(ctx, id)
	}
	ref := entity.FormatStateRef(id, face)
	cascadeCtx := ctx
	if len(res.DeletedRelations) > 0 {
		cascadeCtx = audit.WithTriggeredBy(ctx, "cascade:delete-face:"+ref)
	}
	for _, rel := range res.DeletedRelations {
		m.recordRelationVersion(ctx, store.VersionOpDelete, rel, "", "", "cascade:delete-face:"+ref)
		m.recordRelationAudit(cascadeCtx, audit.OpDeleteRelation, rel, "deleted")
	}
	summary := fmt.Sprintf("deleted face %s", face)
	if len(res.DeletedRelations) > 0 {
		summary = fmt.Sprintf("deleted face %s (cascade: %d relations)", face, len(res.DeletedRelations))
	}
	m.recordEntityAudit(ctx, audit.OpDeleteEntity, current, summary)

	// The face's references are gone; bytes no remaining face references
	// go with them (ruling 4, RR-0SD5ER). A whole-family delete needs no
	// such step: the store removes every byte with the last face.
	if len(metamodel.FileProperties(m.deps.Meta, current.Type)) > 0 {
		releaseUnreferencedFiles(ctx, m.deps, id)
	}

	return &entity.DeleteResult{
		DeletedEntities:  []*entity.Entity{current},
		DeletedRelations: res.DeletedRelations,
	}, nil
}

// faceDeleteEdges collects the edges a delete of id@face is checked against.
// storedLast reports whether face is the family's last stored face, which is
// what the store acts on; callerLast whether it is the last face the caller
// may read, which is what every check acts on.
//
// The checks must not depend on faces the caller cannot read. Judging "last"
// from the stored family answered differently when a hidden sibling existed:
// the cascade opt-in and the authorization of the identity and incoming edges
// ran only without one, so a refusal confirmed there was none. So:
//
//   - Not the caller's last face: the edges TAILED AT IT, which is all the
//     store removes (a readable sibling keeps the entity alive).
//   - The caller's last face, hidden siblings or not: those edges plus the
//     identity edges and every incoming edge, as a family delete takes
//     (RR-2466U1). With a hidden sibling the store keeps the latter, so they
//     are checked but not removed; edges tailed at a hidden face are neither.
//   - The last stored face: every incident edge, which is what the store
//     removes.
//
// tx is the transaction view, so the store's own last-face check, which runs
// under the same serialization, reaches the same answer as storedLast.
func faceDeleteEdges(
	ctx context.Context, tx store.Store, id string, face entity.Face, readableSiblings map[entity.Face]bool,
) (incoming, outgoing []*entity.Relation, callerLast, storedLast bool, err error) {
	siblings, err := store.FamilyHeaders(ctx, tx, id)
	if err != nil {
		return nil, nil, false, false, fmt.Errorf("read the faces of %q: %w", id, err)
	}
	storedLast = len(siblings) == 1
	callerLast = true
	for _, h := range siblings {
		if h.Face != face && readableSiblings[h.Face] {
			callerLast = false
		}
	}
	var tails []*entity.Face
	switch {
	case storedLast:
		tails = []*entity.Face{nil}
	case callerLast:
		identity, own := entity.ImplicitFace, face
		tails = []*entity.Face{&identity, &own}
	default:
		own := face
		tails = []*entity.Face{&own}
	}
	for _, tail := range tails {
		rels, cErr := collectRelations(ctx, tx,
			store.RelationQuery{EntityID: id, Direction: store.DirectionOutgoing, FromFace: tail})
		if cErr != nil {
			return nil, nil, false, false, fmt.Errorf("collect outgoing relations for %q: %w",
				entity.FormatStateRef(id, face), cErr)
		}
		outgoing = append(outgoing, rels...)
	}
	if callerLast {
		incoming, err = collectRelations(ctx, tx, store.RelationQuery{
			EntityID: id, Direction: store.DirectionIncoming,
		})
		if err != nil {
			return nil, nil, false, false, fmt.Errorf("collect incoming relations for %q: %w", id, err)
		}
	}
	return incoming, outgoing, callerLast, storedLast, nil
}

// readableSiblingFaces returns the faces of id other than face that gate
// admits; every one for a nil gate. A gate error is returned.
func readableSiblingFaces(
	ctx context.Context, gate faceReadGate, st store.Store, id string, face entity.Face,
) (map[entity.Face]bool, error) {
	headers, err := store.FamilyHeaders(ctx, st, id)
	if err != nil {
		return nil, fmt.Errorf("read the faces of %q: %w", id, err)
	}
	out := make(map[entity.Face]bool, len(headers))
	for _, h := range headers {
		if h.Face == face {
			continue
		}
		if gate == nil {
			out[h.Face] = true
			continue
		}
		readable, gErr := gate.PermitsReadFace(ctx, h.Type, id, h.Face)
		if gErr != nil {
			return nil, fmt.Errorf("read gate for %s: %w", entity.FormatStateRef(id, h.Face), gErr)
		}
		out[h.Face] = readable
	}
	return out, nil
}

// requireAuthorizedEdges reports an error when deleted holds an edge that is
// in neither authorized set. Edges are matched on their full identity,
// tail face included.
func requireAuthorizedEdges(deleted []*entity.Relation, authorized ...[]*entity.Relation) error {
	ok := make(map[entity.RelationKey]bool)
	for _, set := range authorized {
		for _, r := range set {
			ok[r.Identity()] = true
		}
	}
	for _, r := range deleted {
		if !ok[r.Identity()] {
			return fmt.Errorf("delete face: the store removed %s --%s--> %s, which was not authorized",
				entity.FormatStateRef(r.From, r.FromFace), r.Type, r.To)
		}
	}
	return nil
}

// RenameEntity changes an entity's ID and rewrites all incident
// relations. **No automation, no cascade, no metamodel re-validation
// of the post-rename state** (preserved verbatim from pre-refactor
// workspace behavior).
//
// If opts.DryRun is true, no changes are persisted and no rename record
// is written. A dry run is authorized like the rename, so a denial still
// writes its denied-write record.
func (m *Manager) RenameEntity(
	ctx context.Context, oldID, newID string, opts entity.RenameOptions,
) (*entity.RenameResult, error) {
	// A rename re-keys the whole FAMILY (store.RenameEntity takes no face),
	// hidden faces included, so it is authorized at family level: by a
	// `rename:` grant on a faced type, never by the faces this entity stores
	// (see [Manager.authorizeRename]). An entity none of whose faces the
	// caller may read is reported missing, exactly like an absent one.
	//
	// Fails closed on a non-not-found error: proceeding would run the rename
	// with no authorization at all. An empty family authorizes nothing and
	// falls through to the ErrEntityNotFound the store reports; the Tx below
	// authorizes any face that appears in between.
	family, err := familyRows(ctx, m.deps.Store, oldID)
	if err != nil {
		return nil, fmt.Errorf("rename: load entity %q: %w", oldID, err)
	}
	if rErr := requireReadableRow(ctx, faceGateOf(m), oldID, family); rErr != nil {
		return nil, rErr
	}
	if mErr := requireManualID(m.deps.Meta, family); mErr != nil {
		return nil, mErr
	}
	authorized := make(familyAuthorization, len(family))
	if aclErr := m.authorizeRename(ctx, oldID, family, authorized); aclErr != nil {
		return nil, aclErr
	}
	// Under a read gate the result reports only relations the caller can
	// see. Counted before the rename, because the count must not fail a
	// rename that already happened. Without a gate the store's own count
	// stands: it is taken under the store's lock.
	gate := faceGateOf(m)
	visible := 0
	if gate != nil {
		n, cErr := readableRelationCount(ctx, gate, m.deps.Store, oldID)
		if cErr != nil {
			// The error text can name a neighbor the caller cannot see.
			slog.Error("entitymanager: count visible relations for a rename", "id", oldID, "error", cErr)
			return nil, fmt.Errorf("rename %s: %w", oldID, errRelationCheck)
		}
		visible = n
	}
	if opts.DryRun {
		res, err := renameEntity(ctx, m.deps.Store, oldID, newID, opts)
		if err != nil {
			return nil, err
		}
		if gate != nil {
			res.RelationsUpdated = visible
		}
		return res, nil
	}

	// Collect incident relations (with their content) BEFORE the rename,
	// because the old endpoints are gone afterwards. They feed the per-relation
	// rename versions familyRename.record captures.
	r := familyRename{m: m, oldID: oldID, newID: newID, authorized: authorized}
	if m.deps.RelationVersionRecorder != nil {
		r.preRenameRels = collectRenameAffectedRelations(ctx, m.deps.Store, oldID)
	}

	// Authorize and rename inside ONE Tx, as DeleteEntity does: the store
	// renames the family it finds under its own lock, not the one read above.
	// Version capture, alias rewriting and audit stay outside the callback.
	var (
		res     *entity.RenameResult
		renamed []*entity.Entity
	)
	txErr := m.deps.Store.Tx(ctx, func(tx store.Store) error {
		txCtx := store.ContextInTx(ctx)
		var rErr error
		res, renamed, rErr = r.inTx(txCtx, tx)
		return rErr
	})
	if txErr != nil {
		// pg/sqlite rolled the rename back. fs and memstore cannot, so a
		// denial after the store rename leaves it standing, and the logs must
		// say so (the same reasoning as recordPartialCascade).
		if res != nil && !familyStillStored(ctx, m.deps.Store, oldID) {
			r.record(ctx, renamed)
		}
		return nil, txErr
	}
	r.record(ctx, renamed)
	if gate != nil {
		res.RelationsUpdated = visible
	}
	return res, nil
}

// requireManualID refuses to rename a family whose type generates its ids.
// An unknown type is refused too: nothing declares its ids hand-typed.
func requireManualID(meta *metamodel.Metamodel, family []*entity.Entity) error {
	for _, e := range family {
		def, ok := meta.GetEntityDef(e.Type)
		if !ok || !def.IsManualID() {
			return fmt.Errorf("%w (type %q)", ErrRenameNotSupported, e.Type)
		}
	}
	return nil
}

// familyRename is one non-dry-run RenameEntity: the ids, the faces
// authorized so far, and the incident relations captured before the rename.
// It keeps the rename's Tx body and its logging off Manager.
type familyRename struct {
	m             *Manager
	oldID, newID  string
	authorized    familyAuthorization
	preRenameRels []*entity.Relation
}

// inTx is the rename's critical section. It re-authorizes the family read
// under the transaction, renames it, and then authorizes any face the store
// moved that the re-read did not see: on fs the watcher indexes an external
// file edit without taking the Tx lock, so a face can land between the two.
// It returns the rename result and the moved rows, also alongside an error
// raised after the store rename, so the caller can record a rename a backend
// could not roll back.
//
// The post-rename check still names oldID. On pg and sqlite the ACL's outer
// handle sees the pre-rename graph, so a role conferred through a relation
// still applies; on fs and memstore the relations are already re-keyed, so
// such a role is lost and the face is denied. Both outcomes fail closed, and
// the check only runs for a face no earlier check saw.
func (r *familyRename) inTx(
	ctx context.Context, tx store.Store,
) (*entity.RenameResult, []*entity.Entity, error) {
	family, err := familyRows(ctx, tx, r.oldID)
	if err != nil {
		return nil, nil, fmt.Errorf("rename: load entity %q: %w", r.oldID, err)
	}
	// A face that appeared since the caller's check gets the id-type check
	// too.
	if mErr := requireManualID(r.m.deps.Meta, family); mErr != nil {
		return nil, nil, mErr
	}
	if aErr := r.m.authorizeRename(ctx, r.oldID, family, r.authorized); aErr != nil {
		return nil, nil, aErr
	}
	res, err := renameEntity(ctx, tx, r.oldID, r.newID, entity.RenameOptions{})
	if err != nil {
		return nil, nil, err
	}
	renamed, err := familyRows(ctx, tx, r.newID)
	if err != nil {
		return res, nil, fmt.Errorf("rename: load renamed entity %q: %w", r.newID, err)
	}
	if aErr := r.m.authorizeRename(ctx, r.oldID, renamed, r.authorized); aErr != nil {
		return res, renamed, aErr
	}
	return res, renamed, nil
}

// familyStillStored reports whether any face of id is still stored. A read
// error counts as stored, so a rename is never recorded on a guess.
func familyStillStored(ctx context.Context, st store.Store, id string) bool {
	family, err := familyRows(ctx, st, id)
	if err != nil {
		slog.Error("entitymanager: cannot tell whether a failed rename moved the entity",
			"id", id, "error", err)
		return true
	}
	return len(family) > 0
}

// record writes the audit records and versions of a rename that happened:
// one rename record and one rename version per moved face, since each face
// is its own version lineage, then the alias rewrite and the relation rename
// versions, which are per family.
func (r *familyRename) record(ctx context.Context, renamed []*entity.Entity) {
	m, oldID, newID, preRenameRels := r.m, r.oldID, r.newID, r.preRenameRels
	// No rows means the post-rename read failed (its error is what the
	// caller returns). The per-face records need those rows; the alias rewrite
	// and relation versions below do not, so they still run.
	if len(renamed) == 0 {
		slog.Error("audit.write_failed",
			"stage", "rename-postfetch",
			"new_id", newID,
			"error", "renamed faces could not be read")
	}
	for _, e := range renamed {
		m.recordRenameAudit(ctx, oldID, e)
		// Capture the rename as a version event carrying the old id (prev_id),
		// so a renamed entity's history is walkable back to its former id. Only
		// the choke-point knows old->new; a later sweep sees the renamed entity
		// as an ordinary update and cannot reconstruct this link.
		m.recordEntityVersion(ctx, store.VersionOpRename, e, oldID)
	}

	// Rewrite id-keyed references for the same reason the version above carries
	// prev_id: this is the only point that knows old->new.
	m.rewriteAliasesForRename(ctx, oldID, newID)

	// Capture a `rename` version for each incident relation, on its NEW triple,
	// carrying the pre-rename endpoints (prev_from/prev_to). The version's key is
	// the post-rename endpoint, so WriteRelationVersion resolves the surviving
	// rel_record_id; triggered_by attributes the versions to this rename.
	//
	// Since #1127 the store renames atomically (a bulk in-place
	// `UPDATE relations SET from_id=...`), so the relation KEEPS its
	// rel_record_id across the rename — the lineage is already continuous on one
	// id and this version merely appends a rename MARKER to it (the
	// prev_from/prev_to stitch walk finds no fork; it is harmless belt-and-braces
	// for a future non-atomic path). Capture here is SYNC-ONLY BEST-EFFORT: the
	// atomic re-key does not bump relations.updated_at (see
	// TestRelationRenameDoesNotBumpUpdatedAt), so the reconciliation sweep cannot
	// back-fill a rename this hook misses. That is acceptable — a missed capture
	// loses only the rename marker, never history continuity, because the
	// underlying lineage stays intact on the surviving rel_record_id.
	if len(preRenameRels) > 0 {
		renameTB := "rename-entity:" + oldID + "->" + newID
		for _, rel := range preRenameRels {
			newFrom, newTo := rel.From, rel.To
			if newFrom == oldID {
				newFrom = newID
			}
			if newTo == oldID {
				newTo = newID
			}
			after := &entity.Relation{
				From: newFrom, Type: rel.Type, To: newTo,
				Properties: rel.Properties, Content: rel.Content,
			}
			m.recordRelationVersion(ctx, store.VersionOpRename, after, rel.From, rel.To, renameTB)
		}
	}
}

// collectRenameAffectedRelations gathers the incident relations of id (both
// directions) with their content, for pre-rename version capture. Self-
// referential relations appear once (outgoing). Errors are swallowed — a
// best-effort capture must never fail the rename.
func collectRenameAffectedRelations(ctx context.Context, st store.Store, id string) []*entity.Relation {
	seen := make(map[string]struct{})
	out := make([]*entity.Relation, 0)
	for _, dir := range []store.Direction{store.DirectionOutgoing, store.DirectionIncoming} {
		for r, err := range st.ListRelations(ctx, store.RelationQuery{EntityID: id, Direction: dir}) {
			if err != nil {
				continue
			}
			key := r.From + "\x00" + r.Type + "\x00" + r.To
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, r)
		}
	}
	return out
}

// CreateRelation creates the relation key names, validating endpoints and
// the relation-type tuple against the metamodel. key.FromFace is the tail,
// part of the relation's identity: two edges on one triple with different
// tails are two relations (BUG-64MU2Q). **No automation.**
func (m *Manager) CreateRelation(
	ctx context.Context, key entity.RelationKey, opts entity.RelationOptions,
) (*entity.Relation, error) {
	ctx = withStoreAttribution(ctx)
	from, relType, to := key.From, key.Type, key.To
	// Authorize BEFORE the peer-existence lookups (BUG-K6FEVB). A missing
	// peer must never let a write skip the ACL: if authz is deferred until
	// after GetEntity, a denied caller (e.g. --read-only / ReadOnlyACL)
	// gets a soft "entity not found" instead of a *acl.ForbiddenError,
	// and the dataentry fallback then writes directly to the store,
	// bypassing the ACL and audit. The source type feeds the type-level
	// grant check; it is best-effort (empty if the source doesn't exist
	// yet), mirroring UpdateRelation/DeleteRelation. Authorization must be
	// decided from inputs that don't depend on peer existence.
	source, srcErr := lookupFamily(ctx, m.deps.Store, from)
	// BEFORE the ACL, so an unvalidated coordinate never becomes an
	// authorization coordinate — and so the check still runs on the
	// bypassACL path, where authorizeAndAudit returns without consulting
	// any grant.
	if fErr := m.deps.requireRelationFaceFor(relType, source.typ, key.FromFace); fErr != nil {
		return nil, fErr
	}
	if aclErr := m.authorizeAndAudit(ctx,
		RelationCreateRequest(m.deps.Meta, relType, source.typ, from, key.FromFace)); aclErr != nil {
		return nil, aclErr
	}

	// By family, not GetEntity: an endpoint is an ENTITY and only its type is
	// read here, so asking the zero coordinate would report every faced
	// entity as missing (BUG-HC6I2T).
	//
	// Both fail closed: a transient store error must not be reported as a
	// missing endpoint, which would read as an ordinary validation refusal.
	if srcErr != nil {
		if !errors.Is(srcErr, store.ErrNotFound) {
			return nil, srcErr
		}
		return nil, fmt.Errorf("source %w: %s", ErrEntityNotFound, from)
	}
	target, err := lookupFamily(ctx, m.deps.Store, to)
	if err != nil {
		if !errors.Is(err, store.ErrNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("target %w: %s", ErrEntityNotFound, to)
	}
	if vErr := m.deps.Meta.ValidateRelation(relType, source.typ, target.typ); vErr != nil {
		return nil, fmt.Errorf("invalid relation: %w", vErr)
	}
	// Keyed on the TAIL too: two edges on the same triple with different
	// tails are two relations, so a faced create must not be rejected by
	// the default face's edge (BUG-64MU2Q). Advisory either way — the
	// store's atomic create below is the real guard.
	if _, gErr := m.deps.Store.GetRelation(ctx, key); gErr == nil {
		return nil, fmt.Errorf("%w: %s --%s--> %s", ErrRelationAlreadyExists,
			entity.FormatStateRef(from, key.FromFace), relType, to)
	}

	rel := entity.NewRelation(from, relType, to)
	rel.FromFace = key.FromFace

	tmpl, err := m.deps.Templater.RelationTemplate(ctx, relType)
	if err != nil {
		return nil, fmt.Errorf("load relation template: %w", err)
	}
	if tmpl != nil {
		rel.Properties = templating.ApplyRelation(rel.Properties, tmpl)
	}

	if len(opts.Properties) > 0 && rel.Properties == nil {
		rel.Properties = make(map[string]any)
	}
	maps.Copy(rel.Properties, opts.Properties)
	if opts.Content != nil {
		rel.Content = *opts.Content
	}

	// Auto-assign managed order properties (_order_out / _order_in) when
	// the relation type declares the side orderable. Overrides any
	// non-finite caller-supplied value with AppendOrder over existing
	// siblings; keeps finite caller values as-is. The sibling scan and the
	// create share one Tx so two concurrent appends cannot both read the
	// same last value and land on the same position.
	//
	// CreateRelation, not upsert: a create must never fall through to an
	// update (that would clobber a racing create of the same triple).
	// The GetRelation pre-check above is advisory; the store's atomic
	// create is the real guard, and a conflict surfaces as
	// ErrRelationAlreadyExists (BUG-ZWTDH9).
	create := func(ctx context.Context, st store.Store) error {
		if err := CheckOwningEdge(ctx, m.deps.Meta, st, key); err != nil {
			return err
		}
		if err := m.assignManagedOrder(ctx, st, rel, relType); err != nil {
			return err
		}
		_, err := st.CreateRelation(ctx, key, &store.RelationData{
			Properties: rel.Properties,
			Content:    rel.Content,
		})
		return err
	}
	var createErr error
	// An owning edge reads its neighbors' edges to decide whether it may
	// exist, so the reads and the write share one Tx for the same reason.
	if relTypeIsOrdered(m.deps.Meta, relType) || metamodel.IsOwning(m.deps.Meta, relType) {
		createErr = m.deps.Store.Tx(ctx, func(st store.Store) error { return create(store.ContextInTx(ctx), st) })
	} else {
		createErr = create(ctx, m.deps.Store)
	}
	if createErr != nil {
		if errors.Is(createErr, store.ErrConflict) {
			return nil, fmt.Errorf("%w: %s --%s--> %s", ErrRelationAlreadyExists,
				entity.FormatStateRef(from, key.FromFace), relType, to)
		}
		return nil, createErr
	}
	m.recordRelationAudit(ctx, audit.OpCreateRelation, rel, "created")
	return rel, nil
}

// UpdateRelation merges new properties into the relation key names,
// applies MetaUnset, optionally replaces content, and persists. The tail in
// key addresses the edge exactly, as in [Manager.CreateRelation].
// **No automation, no metamodel re-validation.**
func (m *Manager) UpdateRelation(
	ctx context.Context, key entity.RelationKey, opts entity.RelationOptions,
) (*entity.Relation, error) {
	ctx = withStoreAttribution(ctx)
	from, relType, to := key.From, key.Type, key.To
	// Authorize BEFORE the relation-existence lookup (BUG-K6FEVB): a
	// missing relation must not let a denied caller skip the ACL and get
	// a soft not-found. The source type feeds the type-level grant check;
	// it is best-effort (empty if the source doesn't exist).
	// A lookup error leaves the family zero, whose empty type matches no grant.
	source, _ := lookupFamily(ctx, m.deps.Store, from)
	// BEFORE the ACL, for the reason given in [Manager.CreateRelation].
	if fErr := m.deps.requireRelationFaceFor(relType, source.typ, key.FromFace); fErr != nil {
		return nil, fErr
	}
	if aclErr := m.authorizeAndAudit(ctx,
		RelationUpdateRequest(m.deps.Meta, relType, source.typ, from, key.FromFace)); aclErr != nil {
		return nil, aclErr
	}

	// Reject non-finite numeric values on managed order properties.
	// HTTP wire validators already cover the dataentry path; this is
	// the engine-level backstop for MCP/Lua/CLI write paths.
	relDef, hasDef := m.deps.Meta.Relations[relType]
	if opts.Position != nil {
		return moveRelation(ctx, m, key, relDef, hasDef, opts)
	}
	touchedOut := hasDef && relDef.OutgoingOrderProperty() != "" && touchesOrderKey(opts, relDef.OutgoingOrderProperty())
	touchedIn := hasDef && relDef.IncomingOrderProperty() != "" && touchesOrderKey(opts, relDef.IncomingOrderProperty())
	if hasDef {
		if err := validateOrderUpdate(opts, relDef); err != nil {
			return nil, err
		}
	}

	// Read, merge and write in one Tx: relations carry no version to
	// compare-and-swap on, so without it two concurrent updates naming
	// different properties would each write back the row they read and the
	// first update would be lost.
	var rel *entity.Relation
	var oldProps map[string]any
	err := m.deps.Store.Tx(ctx, func(view store.Store) error {
		txCtx := store.ContextInTx(ctx)
		// Addressed by TAIL as well as triple (BUG-64MU2Q): the default-tail
		// edge is a DIFFERENT relation on a faced source, and the merge below
		// would write the caller's properties onto it.
		var gErr error
		rel, gErr = view.GetRelation(txCtx, key)
		if gErr != nil {
			return fmt.Errorf("%w: %s --%s--> %s", ErrRelationNotFound,
				entity.FormatStateRef(from, key.FromFace), relType, to)
		}

		// Snapshot pre-update meta keys so the audit summary names exactly
		// which keys changed (values never appear).
		oldProps = cloneProperties(rel.Properties)

		if rel.Properties == nil && (len(opts.Properties) > 0 || len(opts.MetaUnset) > 0) {
			rel.Properties = make(map[string]any)
		}
		maps.Copy(rel.Properties, opts.Properties)
		for _, k := range opts.MetaUnset {
			delete(rel.Properties, k)
		}
		if opts.Content != nil {
			rel.Content = *opts.Content
		}

		// UpdateRelation, not upsert: the read above established the triple
		// exists (else ErrRelationNotFound), so this is unambiguously an
		// update (BUG-ZWTDH9).
		_, wErr := view.UpdateRelation(txCtx, key, store.RelationData{
			Properties: rel.Properties,
			Content:    rel.Content,
		})
		return wErr
	})
	if err != nil {
		return nil, err
	}
	m.recordRelationAudit(ctx, audit.OpUpdateRelation, rel, updateRelationSummary(oldProps, rel.Properties))

	// Engine-initiated renumber when an order PATCH collapsed sibling
	// spacing. Errors are operator-visible (slog.Error) but do not fail
	// the user-visible Update — the caller's write already succeeded.
	m.runRenumberAfterUpdate(ctx, from, to, relType, touchedOut, touchedIn)

	return rel, nil
}

// DeleteRelation removes the relation key names. The tail in key addresses
// the edge exactly: a caller that drops the face of a face-tailed edge does
// not delete "roughly the right edge", it names the implicit-tail edge,
// which is a different relation (BUG-64MU2Q).
//
// Only the tail's grammar is checked, not whether the relation type or the
// source type accepts it, so an edge stored under an older schema stays
// deletable.
//
// **No automation.**
func (m *Manager) DeleteRelation(ctx context.Context, key entity.RelationKey) error {
	from, face, relType := key.From, key.FromFace, key.Type
	if err := validTail(face); err != nil {
		return err
	}
	// Authorize BEFORE touching the store (BUG-K6FEVB). The source type
	// feeds the type-level grant check; it is best-effort (empty if the
	// source doesn't exist).
	// A lookup error leaves the family zero, whose empty type matches no grant.
	source, _ := lookupFamily(ctx, m.deps.Store, from)
	if aclErr := m.authorizeAndAudit(ctx,
		RelationDeleteRequest(m.deps.Meta, relType, source.typ, from, face)); aclErr != nil {
		return aclErr
	}
	// Fetch pre-delete AFTER authz (BUG-K6FEVB: a denied delete must return
	// ForbiddenError regardless of whether the relation exists) so the audit
	// record and version snapshot carry the full Subject (relation type + from
	// + to). The relation may not exist — then the store delete returns an
	// error and we skip both the version capture and the audit.
	//
	// Addressed by tail: reading the default edge here would version and audit
	// a different relation than the one being deleted.
	rel, getErr := m.deps.Store.GetRelation(ctx, key)
	// Capture the final pre-delete version BEFORE the store delete, while the
	// live row (and its rel_record_id) still exists — the same order-before
	// rationale as entity delete. Skipped if the relation was already gone.
	if getErr == nil {
		m.recordRelationVersion(ctx, store.VersionOpDelete, rel, "", "", "")
	}
	if err := m.deps.Store.DeleteRelation(ctx, key); err != nil {
		return fmt.Errorf("delete relation: %w", err)
	}
	if getErr == nil {
		m.recordRelationAudit(ctx, audit.OpDeleteRelation, rel, "deleted")
	}
	return nil
}

// relTypeIsOrdered reports whether relType declares a managed order property
// on either side, so creating one reads its siblings.
func relTypeIsOrdered(meta *metamodel.Metamodel, relType string) bool {
	def, ok := meta.Relations[relType]
	return ok && (def.OutgoingOrderProperty() != "" || def.IncomingOrderProperty() != "")
}
