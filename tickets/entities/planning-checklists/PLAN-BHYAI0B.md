---
id: PLAN-BHYAI0B
type: planning-checklist
title: 'Planning: Reconcile endpoint POST /api/v1/_reconcile'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** step 5 of FEAT-CMVZQFI. A peer must be able to ask a rela server
for the count and fingerprint of an address range, and for the elements of a
range, as the calling principal sees them. `visibility.Elements` (TKT-N1KNNZU)
answers one range for the request bound on ctx; nothing serves it over HTTP yet.

**Scope, in:**

- `POST /api/v1/_reconcile`: a list of ranges, each with kind, entity types,
relation types, `lo`, optional `hi` (absent = unbounded), and `op`
(`fingerprint` or `list` with `limit`). The response mirrors the list in order.
- Text codec for the wire: `reconcile.Fingerprint` String/ParseFingerprint and
MarshalText/UnmarshalText; MarshalText/UnmarshalText on `canonical.Digest`
(RR-MDMA4S5).
- Validation: undeclared or duplicate type names (400, naming the type),
relation types on an entity range (400), empty type set (400), bad bounds (400
via store.ErrInvalidQuery), relation meta gated (422 naming the type).
- Budget (RR-ER381BQ, RR-3RVT441): body at most 512 KiB, at most 32 ranges,
at most 256 type names over all ranges, list limits summing to at most 2000 rows
per request, a per-principal token bucket checked first (429 + Retry-After),
then a server-wide slot cap and two slots per principal (503 + Retry-After), and
a request deadline after which the remaining ranges answer `not_run`. The
response is buffered: an error fails the whole request.
- Paging: a range takes `lo` (inclusive) or `after` (exclusive, fed from the
response's `last`); `hi` absent means unbounded. A relation range must name
entity types (the endpoint scope) and relation types.
- `ErrRangeTooLarge` answers that range with `status: too_large` and a hint to
split by type, then by address. Other ranges still answer.
- Wiring: `appbuild.Elements(svc)` builds `visibility.NewElements` with a
DeclarativeGate (or AllowAllReadQueries without a policy), the field policy
(PolicyRedactor or NopRedactor), the roll-up namer and the backend range reader
(pg/sqlite NewElementRanges, elementnaive otherwise); `dataentrywire.Services`
hands it to the app with `dataentry.SetReconcile`.
- archguard: allowlist the appbuild wiring; pin `visibility.NewElements` too.
- OpenAPI entry, CSRF non-browser exemption, route word for the request log,
router walk probe, API reference and acl-security guide sections.

**Scope, out:** the client and `rela reconcile diff` (step 6), peer credentials
and `system:` peers (step 7), apply (step 8), `?world=` (the middleware already
refuses it on `_` routes and non-GET), seeding the gate closure from the range
ids.

**Acceptance Criteria:**

1. Two principals with different read access get different fingerprints for
the same range. Test: ACL fixture where alice reads all tickets and bob only
open ones; same request, different count and fingerprint; bob's list holds no
hidden id.
2. A hidden field changes nothing the reader can observe. Test: a `visible:`
restricted property; changing the hidden value leaves the restricted reader's
fingerprint and element hash unchanged and changes the full reader's.
3. Fingerprint of a range equals the fold of its listed elements (client can
verify). Test: compute from list response with reconcile.Range.Add.
4. Paging: `last`/`more` let a client walk a range. Test: limit 2 over 5 rows.
5. Bad input is refused: undeclared type, duplicate type, bad op, bad kind,
too many ranges, list budget exceeded, bad bounds, unknown field, oversize body.
Test table.
6. Budget: concurrency full gives 503 with Retry-After; rate limit gives 429
with Retry-After.
7. No request identity gives no data (fails closed).
8. The endpoint passes the router security, router walk and OpenAPI tests.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R (linked from FEAT-CMVZQFI).

**Existing Solutions:** Negentropy (range-based reconciliation) is the model;
the server side here is stateless and answers per range.
`golang.org/x/time/rate` (already a dependency) for the token bucket. Patterns:
`batch_handler.go` (strict JSON decode, 1 MiB body), `webhook_routes.go`
(buffered-channel admission, fast 503), `gantt_handler.go` (gate before fold,
never cache across principals), `dataentrywire.Services` (setter wiring).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** a `reconcileHandler` struct in dataentry with a
consumer-side `elementReader` interface (Fingerprint, List), the schema
accessor, an admission channel and a per-principal limiter set. The handler
decodes strictly, validates every range against the metamodel before any read,
admits, rate-limits, then runs ranges in order under a deadline, writing a
result per range. Errors from the store's validation map to 400; other errors to
500 without detail.

Alternatives: the ticket's server-side `match/split/items` answer (server
compares the client's fingerprint and splits). Rejected for v1: the client can
compare locally, and a server-chosen split adds work and a policy (split factor)
to the server. One scope per request instead of per range: rejected, per-range
type sets let the client split by type.

**Files to modify:** internal/reconcile/reconcile.go (codec),
internal/canonical/canonical.go (text marshal),
internal/apiwire/v1/reconcile.go, internal/dataentry/reconcile_handler.go (+
tests), api_v1.go, app.go, middleware_security.go, requeststats.go,
router_walk_test.go, internal/openapi/reconcile.go + paths.go,
internal/appbuild/elements*.go, internal/dataentrywire/wire.go,
internal/archguard/elementrange_test.go, docs/data-entry/api-reference.md,
docs-project GUIDE-acl-security.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the whole body comes from the peer. Type names
are checked against the metamodel (allowlist); kind and op are enums; bounds are
validated by the store (length 4096, UTF-8, no NUL, lo <= hi); counts are
capped.

**Security-Sensitive Operations:** the read itself. Scope is compiled from the
request bound at the edge, never from the body. Results are per principal and
never cached. `Pending` never reaches the wire (wire types carry no such field).
Whether a range is refused as too large depends on scope and row count only.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** handler tests over a test app with an ACL policy and two
principals (AC 1, 2, 3, 4, 7); table test for refusals (AC 5); admission and
limiter tests with injected small caps (AC 6); router walk/security/OpenAPI
suites (AC 8); a dataentrywire test that the wired services serve the route.

**Edge Cases:** empty range (lo == hi), unbounded range, a range outside every
type's window, empty type list, limit 0 / above cap, relation range whose
endpoint types are not named (no rows), range too large.

**Negative Tests:** see AC 5 and 7.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** cost amplification by a peer (mitigated by the budget); a gated type
walks its closure per call (RR-3RVT441; bounded by the rate limit and
concurrency cap); wiring drift between the gate's Declarative and the edge's
(covered by the end-to-end ACL test). Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs/data-entry/api-reference.md (new section),
GUIDE-acl-security (threat model of the endpoint), OpenAPI.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-VMZ3NM0, RR-IHCHB3J (critical); RR-L0WVWGV,
RR-P5HOOIS, RR-3FNL4AN, RR-BDBOB01, RR-YE7U6YW (significant); minor and nit
findings linked on the ticket. All addressed in the plan above.
