---
id: TKT-DN37J2
type: ticket
title: 'ACL: world-shaped read grants, state-shaped write grants (Step 3)'
kind: enhancement
priority: high
effort: l
status: backlog
---

Design doc §8.

- Read grants name worlds (`read: [world:published]`); bare-type and `*` grants mean the default world only — existing acl.yaml files keep their meaning; reading non-default worlds always requires naming them.
- Write grants per `type@pointer` (codec-parsed at policy load), no inheritance, fail closed; no role holds update on published.
- Role resolution needs no new machinery: relation scope replaces v1's `inherits_to_states` (dropped — review found an ancestor-bypass hole in its probe filter). Identity-scoped role relations confer in every world; content-scoped confer on their state.
- Field redaction on state reads fails closed with a global override permission (TKT-73C6B2 family).
- The pushdown read query must express the same world/grant logic as the single-entity path — list and GET verdicts must not diverge.
- Cross-file validation (grants naming declared pointers/worlds, scope names) in `internal/aclaudit`, not `internal/acl` (arch-lint forbids acl → metamodel).

**Owed from Step 2 (TKT-WAV8XP, Q10 decision):** request-level world selection
lands HERE, not in Step 2. Step 2 ships wiring-site binding only — a surface is
constructed over its world and has no world parameter at all. This ticket adds
`?world=` (SPA/API) and `--world` (CLI/MCP) **together with the per-world read
grant check**, because a selectable-but-ungated world parameter is precisely the
partial shipping Jeroen ruled out. Selection is checked against the grant BEFORE
a resolver is constructed; the per-entity visibility gate still runs after; a
principal selecting a world they cannot read gets an empty graph,
indistinguishable from a world with nothing in it. Fail closed, no oracle.

**Also owed (RR-3JRSFV, deferred from PR-A's code review):** tighten the world
NAME charset when this ticket first puts a world name into a URL or a CLI flag.
PR-A validates world names with `ValidateSchemaName`, which rejects the empty
name and the reserved `default` (case-folded) but still admits spaces and
slashes — `world: "a b"` and `world: "world/with/slash"` both load today. The
right charset is a function of the surface that carries it, which is designed
here and not in Step 2, so the restriction was deliberately not guessed at in
PR-A. Tighten it in the SAME change that adds `?world=` / `--world`: after that
ships it is a breaking change for anyone who declared such a name.

**Acceptance criterion (from TKT-T31NKT):** the `world:` grant syntax MUST NOT
merge without the membership-gate load refusal in the same change —
`Policy.Validate` refuses a policy that grants read on a non-default world while
the membership relation is ungated (shared A1 predicate extracted by
TKT-T31NKT), with a hard load error naming the fix and a load-error test.

**Design question to settle in planning (RR-S7A16Q, from TKT-T31NKT's design
review):** the shared predicate `MembershipSelfPromotionOpen` sees only the
direct hole; a CHAINED escalation (membership gated by permission P, but another
ungated role-relation confers a role holding P → self-promotion in two writes)
is invisible to it. Audit A2 flags that shape today, but a load refusal keyed
only on the A1 predicate would inherit the blind spot — a policy could pass the
refusal while still one indirection away from leaking a non-default world.
Decide at planning whether the refusal condition is "A1 open" or "A1 open OR an
A2-shaped chain reaches the membership gate permission"; if the narrower check
is chosen, document why (e.g. A2 stays a high-severity audit finding and the
chain requires an already-privileged grant misconfiguration).
