---
id: TKT-Y72A
type: ticket
title: 'Response-level action affordances: backend declares per-resource verbs to drive UI'
kind: enhancement
priority: medium
effort: xl
status: backlog
---

## Goal

Make the **backend the single source of truth** for which actions a principal
can perform on a specific resource at a specific moment. The SPA renders write
affordances iff the corresponding action is declared in the response.

Today the SPA renders all buttons unconditionally and the backend's 403
reactively explains the deny *after* a failed click. This works for the binary
"read-only mode" case but does not scale: per-type, per-principal, per-state,
per-property authorization all require the SPA to grow its own ACL evaluator
that mirrors the backend's. That's the **duplication trap** TKT-AWM6L's planning
exercise surfaced.

Response-level action affordances (HATEOAS-flavored — `_actions` or `_links` on
each resource) invert the relationship: the server computes "what can this
principal do here" once, attaches it to the response, and the client stays
declarative.

## Why now

PR 1's ACL contract (`acl.ACL.AuthorizeWrite(ctx, req) Decision`) is the
substrate this builds on. The same predicate that decides whether to allow a
write also drives whether to expose its verb in the response.

Three downstream features are unblocked once this lands:

1. **Read-only mode UX** — buttons disappear without per-component frontend gating.
2. **v1 per-type ACL** — Declarative ACL surfaces per-type `_actions`; no SPA changes needed.
3. **v2 per-state workflows** (e.g. "only the assignee can mark a ticket done") — backend computes the verb's availability per-entity; SPA stays declarative.

## Scope (design-first)

This is **xl effort and design-first**. The actual code lands in phases; this
ticket covers the design + initial implementation.

### Phase 1: Research + design doc

`.ignored/action-affordances-design.md` covering at minimum:

- **Wire shape.** `_actions` field vs HAL `_links` vs OData $metadata. Per-resource embedded vs top-level capabilities. Naming.
- **Granularity.** Per-list-item (every row carries `_actions.delete`)? Per-collection-only (list root has `_actions.create`)? Per-property (form spec has `_actions.set` per field)? All three?
- **Computation strategy.** Eager (compute every possible verb per resource) vs lazy (only the verbs the SPA pre-declares interest in). Cost analysis at the prototype's 1k-10k entity scale.
- **Where the computation lives.** In `internal/acl` as new methods? In `internal/dataentry` calling `acl.AuthorizeWrite` per candidate? A new `internal/affordances` package?
- **Frontend consumption.** Per-component check vs centralized resolver. How does this interact with optimistic updates and SSE-driven cache invalidation?
- **Wire compatibility.** Older clients ignore the field. Older servers don't emit it → SPA defaults to "show button optimistically, handle 403 if it comes."
- **Discoverability + introspection.** Top-level `/api/v1/_actions` listing all verbs and their wire shapes? Useful for tooling, MCP, code-gen.
- **Caching implications.** Affordances depend on principal — what's the cache key? Does each user's SPA cache its own set?
- **Read-only mode falls out.** Verify: with ReadOnlyACL, no resource emits any write `_actions`, SPA hides everything. Banner becomes "no actions available" empty state or a tiny dedicated component — not load-bearing.

### Phase 2: Wire shape + backend evaluation

- Define the canonical `_actions` shape across response types (entity, list, relation, form).
- Implement the evaluation point (likely a small helper that wraps `acl.AuthorizeWrite` against a candidate verb).
- Emit `_actions` from one initial endpoint (probably `/api/v1/{type}/{id}`) as a proof-of-shape.
- Wire format docs.

### Phase 3: Full backend coverage

- Every list, entity, relation, form, settings response emits `_actions`.
- Per-property action affordances on form spec (so DynamicForm can de-emphasize read-only fields driven by policy, not by config).
- Top-level `/api/v1/_actions` introspection endpoint.

### Phase 4: Frontend consumption

- SPA components read `_actions` from their fetched resource and render conditionally.
- The 31 affordance sites from PLAN-B0CI's survey become 31 declarative checks against the resource's action set.
- ESLint rule (similar to the abandoned PLAN-B0CI version) enforces that write `@click` handlers reside inside an `<ActionGuard verb="...">` wrapper consuming `_actions`.
- DynamicForm consumes per-property affordances; auto-save short-circuits when the verb isn't declared.

### Phase 5: Read-only mode + docs

- ReadOnlyACL produces empty `_actions` across all resources — verified via E2E.
- `docs/security.md` updates: the SPA's affordance hiding is data-driven, not flag-driven; backend is still the authoritative gate.
- Optional small "Read-only mode" banner if the UX research says the explicit signal is worth it — but this is no longer the driver.

## Out of scope (for v0 of this design)

- **Multi-language i18n of action labels.** v0 emits verb names; UI maps them to copy.
- **Hyper-detailed affordance metadata** (e.g. "you can update this property but only with values from this enum"). The metamodel already provides the value enum; affordances declare verb existence, not constraints.
- **Streaming / SSE delivery of action changes.** v0 affordances are computed at response time; live changes via re-fetch on `refresh` events (consistent with the rest of the SPA).
- **Capability discovery for MCP / external clients.** v1 of the introspection endpoint may add this; v0 of this ticket only serves the SPA.

## Acceptance criteria

Pending design doc completion. Will be filled in during the planning phase per
the standard workflow.

## Process

1. **Research sweep** (parallel agent): HAL, JSON:API, OData, Plone `getActions`, GitHub API `_links`, Kubernetes verbs, Cerbos `principal-permissions`, Stripe affordance hints, HATEOAS thesis. Trade-offs, gotchas, what to copy, what to avoid.
2. **Design doc** from research + the design questions above.
3. **Plan** in this ticket's planning-checklist.
4. **Architect + cranky audits** of the plan.
5. **Crit review** with the user.
6. **Phased implementation** behind feature flag or staged PRs.

## References

- Supersedes: TKT-AWM6L (wont-fix)
- Builds on: TKT-GN5LN (ACL v0 PR 1), DEC-RG878 (four-layer ACL design)
- Survey from PLAN-B0CI: the 31-affordance inventory is still useful as the "which response shapes need `_actions`" inventory
- Original design doc: `.ignored/acl-design.md`
- New design doc (to be written): `.ignored/action-affordances-design.md`
