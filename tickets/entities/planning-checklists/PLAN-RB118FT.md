---
id: PLAN-RB118FT
type: planning-checklist
title: 'Planning: Reconcile client and rela reconcile diff command'
started: "2026-11-07"
completed: "2026-11-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:

- New package `internal/reconclient`: the client side of `POST /api/v1/_reconcile`. A `Differ` runs range-based reconciliation (Negentropy style) between a local element reader and a remote, and returns three sets: only local, only remote, differing (same address, different view hash), plus skipped relation types.
- An HTTP remote with pluggable authentication (`Authenticator`), bounded response size, no redirects, no cookies, typed errors for 429/503 (Retry-After), 422 (relation meta gated) and other statuses.
- `rela reconcile diff --remote <url> [--types ...]` with text and `-o json` output. Token from an environment variable the operator names (`--token-env`) or a file (`--token-file`), never from a flag value and never from a default variable.
- CLI reference docs in docs-project, regenerated docs/.

Out of scope (later steps):

- Fetching the content of changed rows (the step 8 apply side needs it; the diff prints addresses and hashes only).
- Peer identity, stored credentials, `system:<peer>` (step 7).
- `peers.yaml`, applying changes (step 8).

**Acceptance Criteria:**

1. Identical sets: one round trip, empty diff. Test: two memstores with the same rows, remote served by the real handler.
2. Disjoint, overlapping and differing-hash sets are reported correctly, entities and relations. Test: table-driven end-to-end test against an httptest server running the real handler over a second memstore.
3. Paging: remote lists of more than one page are followed with `after=last`; a page whose `last` does not advance is an error.
4. `too_large` answers are split; `not_run` answers are retried; 429/503 honour Retry-After with a capped, bounded wait; 422 marks the relation type skipped with a reason.
5. Malformed answers (wrong range count, bad hex, unordered or out-of-range addresses, addresses of types not asked for, control characters, oversized body) fail the run with an error.
6. Round trips are bounded and pinned by a budget test (10,000 elements, a handful of differences, round trips at most a fixed number).
7. The local side reads through `visibility.Elements` with the CLI's bound identity, never the raw store.
8. `rela reconcile diff` prints the three sets in text and JSON; docs exist in docs-project.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R (range-based reconciliation chosen).

**Existing Solutions:**

- Negentropy (hoytech/negentropy): range-based set reconciliation; each party splits mismatched ranges into buckets at its own element boundaries. We follow it, except the server is stateless and the client drives every split.
- Go libraries: no maintained Go Negentropy client that speaks our wire format; the algorithm is small, so it is written here.
- Codebase: `internal/subscription/fetch.go` (outbound HTTP: timeout, body cap, redirect policy, errors name the origin only), `internal/ai/openai.go` (bearer header). `internal/dataentry/reconcile_handler.go` and `internal/visibility/elements.go` define the contract. `internal/cli/lease.go` for flag and `-o json` conventions; `writeServices.Tokens/TokensErr` for an optional service with a reason.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Package `internal/reconclient`:

- Consumer-side interfaces: `Local` (`Fingerprint`, `List` of `visibility.Elements`) and `Remote` (`Reconcile(ctx, v1.ReconcileRequest) (v1.ReconcileResponse, error)`).
- `New(local, remote, Limits) (*Differ, error)` rejects nil collaborators. `Limits`: max round trips (default 1000), max listed elements (default 1,000,000, local and remote together), overall deadline (default 10 min), max single Retry-After wait (60 s), max consecutive refusals per request (8).
- `Differ.Diff(ctx, Scope)` where `Scope{Entities []string; Relations []RelationScope{Type, Endpoints}}`. It starts with one entity span over all entity types and one relation span over all relation types (endpoint types = union).
- Per round it gathers queries for all open spans, packs them into requests within the server limits (32 ranges, 256 type names, 2000 list rows), and sends them one after the other. Each request is one round trip.
- Fingerprint span: local fingerprint from `Local.Fingerprint` (or in memory once the local elements are known; `ErrRangeTooLarge` locally means list locally). Equal count and sum: done. Otherwise: if remote + local count is small (at most 256), or one side is empty, list; if the span has more than one type, split by type; otherwise list the local span (paged) and split into 16 buckets at local element boundaries, the Negentropy split. Sub-span local fingerprints are then computed in memory.
- List span: page the remote with `after=last` and the local side with `Lo = last + "\x01"`, then merge-join the two sorted lists into the three sets.
- `too_large`: split (type, then local boundaries) or list when the local side has fewer than two elements. `not_run`: ask again next round. 429/503: wait Retry-After (capped), retry the same request, bounded. 422: the problem's `range` member (added to the server for this) names the refused range; split a multi-type relation span by type, or mark a single type skipped with the sanitized reason. A local meta gated type is skipped the same way, with side `local`.
- Every remote answer is validated before use (see Security). Output sorted by (Kind, Address).

HTTP remote `NewHTTPRemote(HTTPConfig{BaseURL, Auth, Transport, Timeout,
AllowInsecureHTTP})`: POST JSON to `<base>/api/v1/_reconcile`; no redirects (a
3xx is an error); no cookie jar; response capped at 16 MiB; per-request timeout
60 s; errors name the origin only. `Authenticator` interface with `BearerToken`
and `HeaderToken` (for a proxy-injected header); token values never appear in
`String`/`%v`.

CLI `rela reconcile diff`: builds `Scope` from `--types` (or all local types)
against the local metamodel; relation endpoints = its From/To types that are in
scope (refused when an end has none); local reader = `appbuild.Elements(svc)`
(the same gated element read the server uses, under the CLI's bound request).
`--exit-code` exits 1 when there are differences.

The command is read-only and runs in the operator's shell, so the outbound HTTP
is not a write-path side effect and does not go on `jobs.Queue`.

Alternatives rejected:

- Lexicographic midpoint splitting of the address space: wastes rounds on common id prefixes (`ticket/TKT-`).
- Listing both sides fully: O(n) transfer even when nothing changed.
- Parallel requests: more server slots per principal; the server caps slots, so sequential requests are simpler.

**Files to modify:**

- `internal/reconclient/*.go` + tests
- `internal/dataentry/reconcile_client_test.go` (end-to-end over the real handler)
- `internal/cli/reconcile.go`, `internal/cli/kong.go` (command field, requiresProject), `internal/cli/cli_wiring.go` (bundle)
- `.go-arch-lint.yml`, `.archplan/config.json` (new component)
- `docs-project/entities/guides/GUIDE-cli-reference.md`, regenerated `docs/`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- Remote answers (untrusted): range count equals request; status in the known set; fingerprint via `reconcile.ParseFingerprint`, hash via `canonical.ParseDigest`; count present, and a zero count has the zero fingerprint; page size at most the limit; addresses non-empty, at most `store.MaxRangeBound` bytes, no control characters, strictly increasing, inside [lo, hi) and after the cursor, and of a type the range asked for; `last` at or past the last element and strictly past the cursor when `more`. Any violation fails the run.
- Response body capped (16 MiB, error bodies 64 KiB); problem detail sanitized (control characters removed, truncated) before it reaches an error or the terminal.
- Remote counts are only used to choose between splitting and listing; all work is bounded by the client's own round-trip, element and time caps.
- User input: `--remote` must be http(s), no userinfo, query or fragment; plain http to a non-loopback host is refused unless `--allow-insecure-http`, with or without a credential. `--types` checked against the local metamodel.

**Security-Sensitive Operations:**

- Credential: read from env or a file (size-capped, trimmed), never a flag value, never logged; the token types redact themselves in fmt.
- No redirects, so the credential is never sent to another origin.
- Local reads go through `visibility.Elements` under the CLI's bound acl.Request, so the diff never shows a row or value the local principal cannot read.
- The remote sees only what its read API already gives this principal (server threat model, step 5).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

- AC1/AC2/AC3/AC7: `internal/dataentry/reconcile_client_test.go`: httptest server running `handleV1Reconcile` (principal bound per request, bearer check) over the fixture memstore; local side `visibility.Elements` over a second memstore. Cases: identical (1 round trip), disjoint, overlapping, differing hash, relation added and removed, hidden field (remote principal with a `visible:` restriction sees a different view hash), more than 2000 rows to force paging.
- AC4/AC5: `internal/reconclient` unit tests with a fake remote over an in-memory element set: too_large splitting, not_run retry, 429 with Retry-After using an injected sleep, max wait cap, 422 skip, malformed answers table.
- AC6: budget test in `internal/reconclient`: 10,000 elements on each side, 0, 1 and 5 differences, round trips pinned.
- AC8: CLI test of the formatter and scope builder; manual run of `rela reconcile diff` against a local rela-server.

**Edge Cases:**

- Empty scope on both sides; one side empty; a range with one local element and many remote; addresses sharing a long prefix; a remote that answers `more` with the same `last`; count changes between fingerprint and list (concurrent writes): the list is authoritative.

**Negative Tests:**

- Wrong range count, unknown status, bad hex, unsorted addresses, address out of range or of another type, control characters, oversized body, 3xx, 401, 400 with detail, 429 beyond the retry cap, deadline exceeded, round-trip cap exceeded.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Local and remote disagree on element encoding: both use `reconcile`/`canonical` and the same `visibility.Elements`; the identical-set test proves one round trip.
- Unbounded work on a hostile or broken remote: client caps on round trips, elements, deadline and retries.
- A remote without the `range` member: with one relation range in the request it is that one; otherwise the run fails with a protocol error.
- The remote sees local addresses used as split bounds: small spans are listed instead of split, and the parts of a split must add up to the remote's count of the whole.

Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/cli-reference.md (via docs-project GUIDE-cli-reference): new `rela reconcile diff`

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 17 findings, recorded as review responses:
RR-QR7Y7HP, RR-U6M954C, RR-TMDW8OD, RR-LMKYGDF, RR-14HP4KA, RR-UX2AKWN,
RR-L8J6X1I, RR-7AG0GZS, RR-U1FSOTL, RR-MAQQ2AQ, RR-USDQOWH, RR-4PZGCO4,
RR-RSMNFI0, RR-OSSK9G8, RR-X7B3KHZ, RR-M7HPZ9G, RR-7QXWY1O. The plan above
includes their fixes: local meta gating, a `range` index on the server's
per-range problems, a cap on not_run, chunking under the 256 type name cap, one
stable endpoint scope for relations, buckets sized under the row cap after
too_large, separate local and remote caps, explicit paging rules, a three-part
relation address parse, view semantics in docs, a CLI hint for a missing
identity, `--exit-code` 1 and 2, terminal sanitization, credential edge cases,
and a transport subpackage.
