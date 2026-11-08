---
id: PLAN-P1013IE
type: planning-checklist
title: 'Planning: Range fingerprints for types with redacted fields'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Step 4 of FEAT-CMVZQFI. The step 3 range readers fold the stored element of a
row only when the reader has no hidden field on it; every other admitted row
comes back pending. This step completes pending rows with the reader's view
hash, so a range answer is complete for that reader, and compiles the reader's
`store.ElementScope` from the bound ACL request so the stored and the redacted
paths share one source of truth. It also resolves RR-3HN68CI (bodies in the view
hash).

**Scope:**

In scope:
- `internal/visibility/elements.go`: `Elements` (the per-reader element range read) with `Scope`, `Fingerprint` and `List`, and the opaque `ElementScope` it returns.
- Scope compilation from the bound request: row gate from `ReadQueryProvider.ReadQueryFor` (as `Searcher.scope`), field visibility from `FieldScoper.SearchFieldScope` through the existing `visibleWhen`, roll-up names as hidden fields with an empty condition, relation types with meta `visible:` grants refused.
- Completion of pending rows: list the range once, batch-load the pending rows per type, prime and redact each once through `Redact`, drop roll-up names, hash with `canonical.SumEntity` / `SumRelation` (the ETag function), fold.
- `affordances.PolicyResolver.RelationMetaGated(relType)` and the matching `PolicyRedactor` / `NopRedactor` methods.
- `canonical.ParseDigest` (the first reader of stored hex hashes; partly answers RR-MDMA4S5).
- Archguard allowlist entry for `internal/visibility/elements.go`; arch-lint: visibility may depend on reconcile and canonical.

Out of scope:
- The HTTP endpoint, wire format, splitting strategy and rate limit (step 5).
- Production wiring of `Elements` in appbuild (step 5 adds it with the handler).
- Relation meta redaction in the view hash (TKT-0RBFN0 territory): a relation type with meta `visible:` grants is refused by `Scope`, never folded raw.
- A stored body digest (rejected, see RR-3HN68CI below).
- RR-DR4K5RK (sqlite statement counter): stays deferred, not cheap.

**Acceptance Criteria:**
1. A change to a hidden field does not change the fingerprint a reader gets (real policy, memstore + elementnaive).
2. The redacted and unredacted paths agree when the reader has no hidden field: the completed fingerprint of rows forced pending equals the stored-path fingerprint for the same view.
3. A reader who sees a field gets a different fingerprint when that field changes; two principals with different field access get different fingerprints for the same range.
4. Budget: one `Fingerprint` and one `List` cost the same number of store calls at 10 and at 50 pending rows (`storetest.Counting`).
5. A scope compiled for one principal is refused under another principal's ctx.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R (compute the view hash of the redacted projection
in Go, one page at a time).

**Existing Solutions:**
- `visibility.Searcher.scope` and `withFieldScopes` (internal/visibility/search.go): ACL read query and field scope per type, compiled once per request. Same compile here.
- `visibility.visibleWhen` (rollup.go): FieldScope to one RowCondition per gated property; step 3's plan named it as the converter.
- `PolicyReader.redactPage` / `primePage` (pushdown.go): prime local roles and traversals for a page, then `Redact` per row. Same here.
- Gantt aggregate pattern (CLAUDE.md): row-gate, redact each entity once, then fold; per principal, never cached.
- `dataentry.servedETag` / `entityView`: the ETag hashes `canonical.SumEntity` of the row minus hidden properties; `Redact` produces the same view.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

`visibility.Elements` is built from a capability bundle `ElementDeps`: the store
range reader (consumer-side interface: `RangeFingerprint`, `ListRange`), a row
loader (`ListEntities`, `ListRelations`, `MatchingFaces`), `ReadQueryProvider`,
one `ElementPolicy` (`SearchFieldScope`, `HiddenProperties`,
`RelationMetaGated`, implemented by `PolicyRedactor` over one resolver, so the
stored-path decision and the completed view cannot come from different policies)
and a roll-up namer. All are required. (Design review RR-OE4GAV0.)

The caller names the types (`ElementTypes{Entities, Relations}`); every
`Fingerprint` and `List` call compiles the store scope from the request bound on
ctx. No scope value is exported, so nothing can keep one across requests
(RR-P1FK0IN). Compile:
- Per entity type: `ReadQueryFor`; DenyAll or an empty non-nil face list omits the type; AllowAll or Query with Faces maps to `store.ElementTypeScope`; a result with no branch set is an error. `SearchFieldScope`: nil means nothing hidden; otherwise each gated property becomes a `HiddenField` with `visibleWhen`'s condition, skipped when some grant shows it on every row (nil When). Each roll-up name is added with an empty condition (deduplicated; the empty condition wins).
- Per relation type: `RelationMetaGated` (policy-wide, not per principal) true is `ErrRelationMetaGated` naming the type; otherwise `&FieldVisibility{}`.

`Elements.Fingerprint(ctx, types, r) (reconcile.Range, error)`:
- `store.PlanElementRange` (validation, fold modes). When every type in play folds stored (entity types for an entity range, relation types for a relation range), one `RangeFingerprint`; with no pending row (pending can then only mean a NULL stored element), that is the answer.
- Otherwise list the range once (`ListRange`, limit max+1). More than max rows is `ErrRangeTooLarge`. The decision uses the row count and the plan only, never the pending count.
- In the listing, every row of a type whose fold is not FoldStored is completed in Go, pending or not, so the work does not depend on whether a hidden field is set (RR-3TYBXP3). Rows of FoldStored types fold their parsed stored hash, or are completed when their element is NULL. Count and sum come from the folded rows.

`Elements.List(ctx, types, r, limit) ([]reconcile.Element, error)`: limit
1..max; one `ListRange`, completed by the same rule.

Completion of one listing: entity entries grouped by type, one
`ListEntities{Type, IDs, AllFaces}` per type, matched by ref, then checked again
against the type plan's gate (one `MatchingFaces` per gated type) and face
allowlist (RR-7B4BLLB); `primePage` then `Redact` once per row; roll-up names
removed; `reconcile.EntityElement`. Relation entries (only NULL-element rows)
grouped by type, one `ListRelations{Type, EntityIDs: froms, PeerIDs: tos,
Outgoing}` per type, matched by key, `reconcile.RelationElement` on the raw row;
they keep the listing's endpoint verdict (documented). A row gone at load or
failing the gate is dropped and not counted. Stored hashes are from list time,
completed hashes from load time (godoc).

RR-3HN68CI (bodies): the view hash keeps the canonical encoding, which streams
the formatted body into SHA-256. A pending row is therefore loaded with its body
and formatted, a page at a time, with the work per call bounded by the row cap
(default 2000, option). Rejected: a stored body digest with a new element
encoding; the element hash would then differ from the ETag view hash and from
content_hash, which step 1 made equal on purpose. Bodies are never
policy-hidden; the completion goes through `Redact`, so if body redaction is
added the completed hash follows, and the hidden-body condition the store scope
then needs is already on the TODO(body-redaction) list.

Alternatives rejected:
- Compile the scope in step 5: the stored path's correctness depends on the scope matching `Redact`; testing the completion against a hand-built scope cannot show that.
- A pending-count cap: refusing a range because it has many pending rows discloses that hidden fields are set there.
- Complete relation meta redaction now: the source row whose grants apply is ambiguous for an identity tail on a faced source (TKT-0RBFN0).

**Files to modify:**
- `internal/visibility/elements.go` (new), `elements_test.go`, `elements_budget_test.go` (new)
- `internal/visibility/adapters.go` (RelationMetaGated on PolicyRedactor / NopRedactor)
- `internal/affordances/resolver.go` (RelationMetaGated) and a test
- `internal/canonical/canonical.go` (ParseDigest) and a test
- `internal/archguard/elementrange_test.go` (allowlist)
- `.go-arch-lint.yml` (visibility may depend on reconcile, canonical)
- `internal/reconcile/reconcile.go`, `internal/store/elementrange.go`, `internal/visibility/policyreader.go` (docs)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Range bounds come from a peer in step 5; `store.PlanElementRange` validates them. Limit is checked 1..max.
- The scope is compiled from the bound request only; the opaque type makes a hand-built scope impossible outside the package.

**Security-Sensitive Operations:**
- Gate before fold: rows come only from the scope-gated listing; each pending row is redacted exactly once from a raw store row, then hashed, then folded. Per principal, never cached.
- No pending signal leaves the package: the result is a count and a sum; the too-large refusal depends on the row count and the plan only. Residual: completion time grows with the pending rows (timing), as the ETag path does; documented.
- Errors name types and counts, never row content.
- Scope compiled for principal A is refused under principal B.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1/AC3: real `acl.Policy` + `affordances.New` + `PolicyRedactor` + memstore + elementnaive (as pushdown_prime_budget_test.go). Reader alice without `salary`; change salary: alice's fingerprint unchanged, an admin's changes; change title: both change. Conditional grant (`when:`) with rows on both sides.
- AC2: a reader with no hidden field: completed fingerprint of the same rows (forced pending by a roll-up name that is absent from them, or by a scope with unknown visibility) equals the stored-path sum; each completed element equals `reconcile.EntityElement` of the redacted row; `List` equals the elements `Fingerprint` summed.
- AC4: `storetest.Counting` under memstore at 10 and 50 pending rows, `Fingerprint` and `List`, entity and relation ranges.
- AC5: compile under alice, call under bob: error.

**Edge Cases:**
- Empty range, denied type, zero `ElementScope`.
- Roll-up name stored on a row: pending, completed view without it.
- Row deleted between list and load: dropped.
- Relation type with meta grants: refused by `Scope`.

**Negative Tests:**
- Limit 0 / above max; range over max rows on a per-row type; principal mismatch; meta-gated relation type; nil deps in the constructor.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Scope compile drifting from `Redact`: both come from the same resolver (`SearchFieldScope` is pinned to `FieldVerdicts` by `TestSearchFieldScope_AgreesWithFieldVerdicts`); the AC1/AC2 tests check the end-to-end equality.
- Body formatting cost: bounded per call by the row cap.
- Clients cannot fingerprint a large range of a per-row type in one call: step 5 splits.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed (godoc carries the contract; the endpoint docs come with step 5)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** see review responses linked to TKT-N1KNNZU.
