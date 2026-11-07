---
id: PLAN-9VXK011
type: planning-checklist
title: 'Planning: Reconciliation element and reader view hash'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

Step 1 of FEAT-CMVZQFI. Define the reconciliation element (address, view hash),
the fixed-width additive element fingerprint and the range fingerprint `{count,
sum}`, in a leaf package. Make the entity and relation ETags use the same
view-hash function.

**Scope:**

In scope:
- New leaf package `internal/reconcile` (imports only `entity` and `canonical`).
- `canonical.SumEntity` / `canonical.SumRelation` returning the raw 32-byte
digest; `HashEntity` / `HashRelation` become hex of those (values unchanged).
- Entity ETag (`entityETagWithEdges`) rebuilt on `reconcile.EntityViewHash`
of the redacted view, plus the served edges. Relation ETag
(`relation_read_handler.go`) calls `reconcile.RelationViewHash`.
- Arch-lint component for the new package.

Out of scope (later tickets):
- Storing the hash or fingerprint (TKT-1G2XR1R), store range capability,
the redacted page path, `_reconcile`, wire encoding of fingerprints, ordering of
the key space, checking that relation endpoints are readable (the caller's job;
documented on `RelationElement`).

**Acceptance Criteria:**
1. One function produces the view hash, and both ETags use it. Test: the
relation ETag equals `reconcile.RelationViewHash(view).String()`; the entity
ETag is derived from `EntityViewHash(view)` plus the edges, and the existing
ETag tests (BUG-J01V0Q `etag_redaction_test.go`, world/face ETag tests, CAS
tests) pass unchanged.
2. A redacted field does not change the view hash. Test: two entities that
differ only in a property removed from the view hash equal; the `Redacted` and
`Inaccessible` lists do not change the hash; the existing
`TestEntityETag_ExcludesHiddenFields` still passes.
3. The fingerprint is pinned. Test: golden values for a fixed entity and
relation element, so a change to the encoding fails the test.
4. Range fingerprints are additive. Test: Add/Remove/Merge are order
independent, wrap modulo 2^128, and Remove inverts Add.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R (linked to FEAT-CMVZQFI).

**Existing Solutions:**
- `internal/canonical`: the cross-backend content hash (length-prefixed
SHA-256, normalized values). The view hash reuses it, so a reader without
redacted fields gets exactly the stored `content_hash`.
- `internal/dataentry/relation_read_handler.go:78` already hashes the
redacted relation view with `canonical.HashRelation`.
- `internal/dataentry/api_v1.go` `entityETagWithEdges` hashes with its own
`%v` encoding; it is replaced.
- `entity.RelationKey.String()` gives the stable relation address
(`from[@face]--type--to`), `entity.FormatStateRef` the `id[@face]` form.
- Negentropy (range-based set reconciliation) uses an additive sum of element
hashes mod 2^256 plus the count. No Go library fits: we need the SQL-side sum,
so the element and fingerprint are ours.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach (revised after design review):**

- `canonical`: `type Digest [32]byte` with `String()` (lower hex);
`SumEntity` / `SumRelation` return it; `HashEntity` / `HashRelation` are
`Sum*(..).String()`, values unchanged. The view hash is `canonical.Sum*` applied
to the reader's view: one function for ETags and reconciliation.
- `reconcile` holds only Element, Kind, Fingerprint and Range and takes a
`canonical.Digest`. dataentry does not import it.
- `Kind` values are the bytes 'E' and 'R'; ordering key is (Kind, Address),
bytewise; a range never spans kinds; SQL compares with a byte collation.
- `Fingerprint.Limbs() [4]uint32` and `FingerprintFromLimbSums([4]uint64)`
define the SQL storage and summation form.
- `Range.Count` wraps mod 2^64.
- Entity ETag = SHA-256 over the view digest, the raw served body and the
sorted edge keys, all length-prefixed (the raw body keeps formatting-only edits
visible to If-None-Match / If-Match).
- The fingerprint detects accidental difference only; a writer in scope can
craft a cancellation at any width.

Original draft for reference:

Package `internal/reconcile`:
- `type ViewHash [32]byte` with `String()` (lower hex, equal to
`canonical.HashEntity`).
- `EntityViewHash(view entity.Entity) ViewHash` = `canonical.SumEntity`;
`RelationViewHash(view entity.Relation) ViewHash` = `canonical.SumRelation`. The
argument is the row as the reader sees it: `visible:`-hidden values are already
removed (visibility.Redact, or the handler's redaction). The hash never covers
`Redacted`, `Inaccessible` or `UpdatedAt`.
- `type Kind byte` (`KindEntity`, `KindRelation`), `type Element struct
{Kind; Address string; Hash ViewHash}`.
- `EntityElement(view)` with address `type/id[@face]`;
`RelationElement(view)` with address `RelationKey.String()`.
- `type Fingerprint struct{Hi, Lo uint64}` (N = 128). `Element.Fingerprint()`
= first 16 bytes, big endian, of SHA-256 over a domain tag `rela/reconcile/v1`,
the kind byte, the length-prefixed address and the 32 hash bytes.
`Fingerprint.Add/Sub` wrap mod 2^128.
- `type Range struct{Count uint64; Sum Fingerprint}` with `Add(Element)`,
`Remove(Element)`, `Merge(Range)`; comparable with `==`.

ETags:
- Entity: `entityETagWithEdges(e, hidden, edges)` builds the view (copy of e
without hidden properties), takes `EntityViewHash`, then SHA-256 over that hash
and the sorted, length-prefixed edge keys. Output format unchanged (quoted
base64 of 8 bytes). Values change once; ETags are opaque.
- Relation: `reconcile.RelationViewHash(view).String()`; the value is
byte-identical to today.

**Alternatives:**
- N = 64: one SQL column, but Wagner's generalized birthday attack lets a
writer cancel a change with about 128 crafted rows. 128 bits needs about 32,768
rows; summing in SQL uses 32-bit limbs either way. 256 bits (as Negentropy)
costs four columns for little more.
- Hex string view hash: wasteful for the fingerprint; the 32-byte array keeps
hex as the display form only.
- Keeping the separate entity ETag encoding: violates the acceptance
criterion and keeps the weaker `%v` encoding.

**Files to modify:**
- new `internal/reconcile/{reconcile.go,reconcile_test.go}`
- `internal/canonical/canonical.go` (+ test)
- `internal/dataentry/api_v1.go`, `internal/dataentry/relation_read_handler.go`
- `.go-arch-lint.yml`, `.testcoverage.yml` if a floor is needed

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Rows come from the store via the caller; no parsing of untrusted input in
this ticket. Length prefixes keep the encoding unambiguous for any bytes.

**Security-Sensitive Operations:**
- The view hash must cover only what the reader sees (BUG-J01V0Q): the
function takes the view, and the ETag code redacts before hashing, as now. It
does not cover the names of redacted fields, so whether a hidden field is set
does not show.
- The additive fingerprint is not collision resistant against a writer who
crafts rows. It detects accidental difference; 128 bits raise the cost of a
crafted cancellation. Documented in the package doc.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: relation-read test asserts the ETag equals
`RelationViewHash(redacted view)`; entity ETag tests in dataentry run through
the HTTP handlers (integration).
- AC2: unit tests in reconcile; `TestEntityETag_ExcludesHiddenFields`.
- AC3: golden fingerprints.
- AC4: table tests on Range.

**Edge Cases:**
- Empty properties vs nil map hash the same (canonical).
- Faceless vs faced entity: face in address and hash.
- Fingerprint wraparound at 2^128 (carry from Lo into Hi, overflow of Hi).
- Empty range is the zero value.

**Negative Tests:**
- Different kind with same address and hash gives a different fingerprint.
- Changing a visible property or the body changes the view hash.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Entity ETag values change on deploy: clients holding an old ETag get one
412 or a full 200. Acceptable; ETags are opaque.
- The canonical body normalization means a whitespace-only body change that
FormatMarkdown folds away keeps the entity ETag. fsstore already stores the
formatted body; the relation ETag already behaves this way.
- Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed (godoc only).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-R6ZK1SK (addressed), RR-I6TTK56 (addressed),
RR-IIGONJR (addressed), RR-3HN68CI (deferred to step 4 with reason), RR-U081VJ9,
RR-2BV7V1O, RR-QYKY2XH, RR-XDCLMHW, RR-IX7IWMK, RR-8CQMKV1, RR-MY0HTGC
(minor/nit, addressed).
