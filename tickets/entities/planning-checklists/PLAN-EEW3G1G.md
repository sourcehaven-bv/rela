---
id: PLAN-EEW3G1G
type: planning-checklist
title: 'Planning: POST /api/v1/_batch endpoint, OpenAPI and API reference'
started: "2026-11-04"
completed: "2026-11-04"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: `POST /api/v1/_batch` on `entitymanager.CreateBatch`, its OpenAPI
entry, the problem+json `operation` extension, the CSRF non-browser exemption,
and the api-reference docs.

Out of scope: updates, deletes and relation properties in a batch (strict decode
rejects unknown fields), an MCP tool (DEC-ODU65KR item 3), configurable limits
(item 4), atomic `POST /{plural}` with relations (TKT-H46UGLM), and
draft-protocol docs for fs/memory (TKT-HHHU76J).

**Acceptance Criteria:**

1. On fs/memory (`capabilities.atomic_batch` false) every request is 501
`atomic_batch_unsupported`, before the body is read.
2. A body over 1 MiB is 413 `body_too_large`; more than 100 operations is 400
`batch_invalid`.
3. A duplicate ref, an unknown ref, an endpoint with both or neither of
`ref`/`id`, or an unknown field is 400 `batch_invalid` with a JSON pointer
(`/entities/2/ref`, `/relations/0/to/ref`). A ref is only defined by an entry in
`entities`, and every entity is written before any relation, so a "ref used
before defined" is the same error as an unknown ref.
4. 200 lists each entity with its `ref`, `index`, generated `id` and the
entity redacted for the caller, plus `warnings` and `follow_up_error`; it lists
the created relations.
5. A relation to a hidden existing entity answers exactly what a relation to
a missing id answers: 422 `target_not_found`, the same pointer, the same body.
6. A type-disallowed edge fails the whole batch (422
`source_type_not_allowed` / `target_type_not_allowed`), nothing written.
7. A denied create or relation is 403 with `operation {kind, index, ref}`.
8. A failure inside the transaction on sqlite/postgres leaves no rows and
answers with the failing operation.
9. OpenAPI has the path with 200, 400, 403, 409, 413, 422, 501.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-QKREXZZ, DEC-ODU65KR.

**Existing Solutions:**

- Wire shape follows JSON:API atomic operations' `lid` idea (here `ref`).
- `handleV1CreateEntity` (internal/dataentry/write_handler.go) is the
per-entity template: `parseCreateOpts`, `createFace`, `validateCreateIDOpts`,
`gateCreateRelationAffordances`, `requireReadablePeers`/`danglingPeerError`,
`ClassifyCreateError` mapping, `writeIdentityFaultDeny` for identity faults.
- Body limit: `http.MaxBytesReader` and 413 `body_too_large` as elsewhere in
dataentry. Strict decode with `DisallowUnknownFields`.
- OpenAPI: `internal/openapi/leases.go` pattern.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Handler `handleV1Batch` in a new internal/dataentry/batch_handler.go:

1. `withProvision`, then 501 if the app's atomic flag is off.
2. `MaxBytesReader` at 1 MiB, strict decode into `v1.BatchRequest`.
3. Shape: operation count, ref uniqueness and syntax, each endpoint exactly
one of `ref` or `id`, refs resolve to an entity index; each error carries a JSON
pointer. A `face` on a `ref` end or on a head is refused.
4. Per entity: resolve the type by its `type` field, face and custom id
checks as the single create does.
5. One batched `readableTypes` call for every existing endpoint id. A missing
or hidden id is 422 `target_not_found` with the pointer to that endpoint, the
same body for both.
6. Relation affordance verdicts per relation, as the single create's
`gateCreateRelationAffordances` does, so the batch denies exactly what single
requests deny (audited through `denyAfford`).
7. `entitymanager.CreateBatch` through the field-gated manager handle.
8. Error mapping: `BatchOpError` gives `operation {kind, index, ref}`; the
inner error goes through the same classes as the single create (403, 409, 422).
Identity faults go through `writeIdentityFaultDeny` first.
9. 200 response; each entity serialized with `forWireRelated` (property
redaction for the caller; no relations, no rollups).

Wiring: route in `registerAPIV1Routes`, `//archplan:auth=authz`, a probe in
router_walk_test.go, the route word in requeststats.go, `_batch` in the
non-browser CSRF exemption (same no-cookie, no-Origin shape as the data routes;
a browser caller still needs same-origin).

Alternatives rejected: batching through repeated `POST /{plural}` (not atomic);
a JSON Patch body (no ids for new rows); 207 multi-status (the batch is
all-or-nothing, so one status is the truth).

**Files to modify:**

- internal/dataentry/batch_handler.go, batch_handler_test.go (new)
- internal/dataentry/api_v1.go, middleware_security.go, requeststats.go,
router_walk_test.go
- internal/apiwire/v1 (request, response, `Error.Operation`)
- internal/openapi/batch.go, batch_test.go (new)
- docs/data-entry/api-reference.md (hand-maintained; no docs-project
source exists for this file)

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

The HTTP body (untrusted). Size capped before decode; unknown fields rejected;
operation count capped; refs are opaque labels compared as strings and never
reach the store or a path; types, properties, faces and ids go through the same
validation as single creates.

**Security-Sensitive Operations (threat model, POLICY-015 §3 / PROCEDURE-j5mq
step 1):**

- ACL per operation: every entity create runs the create ACL and field gate,
every relation the relation ACL and the relation affordance verdicts, all as the
bound caller, before the transaction. No batch-level grant exists.
- Hidden endpoints: existing ids are checked in one gated read; missing and
hidden give the same 422 `target_not_found` body and pointer. The engine repeats
the check and also treats hidden as missing. A test compares the two responses
byte for byte.
- Created rows the caller cannot read (DEC-ODU65KR item 5): the response
returns the id (the caller just created it, so the existence is not a secret to
them) and redacts property values the caller may not see.
- Limits: 1 MiB and 100 operations bound decode cost and the time the write
lock is held. Fixed, not configurable.
- Audit: after commit only; a denial audits as `denied-write` like single
creates; a rolled-back batch audits nothing.
- Id minting under the lock: done in the engine inside the Tx.
- Partial commit per backend: 501 on fs/memory, so the endpoint never runs
where the store cannot roll back.
- CSRF: `_batch` joins the non-browser exemption on the same terms as the
data routes; a cookie-bearing request keeps the same-origin check.
- Errors name the operation index and ref (caller-supplied), never another
principal's data.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

internal/dataentry/batch_handler_test.go (default build uses fsstore with the
flag forced on for handler logic where rollback is not asserted; the rollback
assertion runs under `-tags sqlite`):

1. AC 1: flag off gives 501.
2. AC 2: 101 operations gives 400; a body over 1 MiB gives 413.
3. AC 3: table of duplicate ref, unknown ref, both/neither endpoint keys,
unknown field, with the expected pointer.
4. AC 4: a batch of two entities and an edge between them; the edge's ends
are the generated ids; a hidden property is redacted in the response.
5. AC 5: hidden vs missing endpoint, identical status and body.
6. AC 6: type-disallowed edge, nothing written.
7. AC 7: ACL denies the type; 403 with `operation`.
8. AC 8: sqlite, duplicate edge in the batch, no rows after.
9. AC 9: openapi test with `requireResponses`.

**Edge Cases:** empty batch (400), ref of an empty string (400), relation only
batch between existing entities, faced tail on an existing source, custom id on
a batch entity, unique clash inside the batch.

**Negative Tests:** the 400/403/413/422/501 cases above.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Drift between the batch checks and the single create's checks.
Mitigation: reuse the same helpers, not copies.
- The CSRF exemption widens what a non-browser client may call. Mitigation:
same conditions as the data routes, documented at the list.
- Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/data-entry/api-reference.md: a new `POST
/api/v1/_batch` section, a rewritten Atomicity paragraph (the store has `Tx`
since DEC-8UIL0) and the "Out of scope" line on cross-entity transactions.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-E3IPT8D (critical), RR-SL3ADIC, RR-MNYZCY0
(significant), RR-CRIOYOZ, RR-RLMZYQY (minor), RR-IHWNWQQ, RR-CRVAETI,
RR-6VCPH64 (nit). All addressed.
