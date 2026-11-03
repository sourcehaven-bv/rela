---
id: PLAN-7FYKKQ
type: planning-checklist
title: 'Planning: Access PR 1: golden wire-parity corpus, test ACL helpers, verifiedIdentity rename'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem understood
- [x] Scope defined
- [x] Acceptance criteria written

**Problem:** TKT-WFS778 moves every ACL, affordance and gated-read decision in
dataentry behind `internal/access` over five PRs, each promising "no behaviour
change on any endpoint". Nothing records today's wire behaviour per principal,
so that promise cannot be checked. And 366 test sites assign `app.acl` /
`app.fieldResolver` directly; PR 6 removes those fields, which would mean
touching every site again in the riskiest PR.

**Scope:** In: (1) golden wire-parity corpus test in internal/dataentry; (2)
`setTestACL` / `setTestResolver` helpers and a mechanical rewrite of all direct
assignments in dataentry tests; (3) rename the `jwtWired` parameter of
`acl.Policy.RejectEffective` and its caller to `verifiedIdentity`. Out: any
production code change beyond the rename; access package; the five type
switches; BUG-2TPO2F / BUG-HZ4FHZ (the corpus records their current behaviour,
and the fix PRs update the affected golden files deliberately).

**Acceptance Criteria:**
1. `go test ./internal/dataentry -run TestGoldenParity` passes and reads its expectations from `internal/dataentry/testdata/golden/`; `-update` rewrites them. Test: run twice with no change, and once after a deliberate one-field change to the fixture's acl.yaml to see it fail with a readable diff.
2. Each case records status, selected headers, normalized body bytes and store reads (calls by method, breadth, body loads) as seen through production wiring. Test: the golden files contain all three.
3. The corpus covers at least four principals (admin, editor, viewer with hidden rows and hidden `when:` fields, outsider with no grants) against list, detail, relations, views, nav/menu, dashboard, gantt, search, export, attachments, history, `_actions`/affordances, and the write endpoints (create, update, relation create, delete), each write on a fresh app. Test: case table review; failclosed and facegate behaviours appear in it.
4. Output is deterministic: 20 consecutive runs (`-count=20`) and runs under `-race` produce no diff.
5. No `app.acl =` or `app.fieldResolver =` assignment remains in dataentry tests outside the two helpers and the documented wiring sites. Test: grep; full dataentry suite passes unchanged.
6. `jwtWired` no longer appears in any `*.go` file (ticket history keeps it). Test: grep; acl and dataentry suites pass.

## Research

- [x] ~~Research document created~~ (N/A: design already done in .ignored/TKT-WFS778-design.md and the archplan thread th-7f905a03)

## Approach

- [x] Codebase explored
- [x] Approach documented
- [x] Files to modify listed
- [x] Alternatives considered

**Existing Solutions:** `storetest.Counting` (read counts, already used by gantt
and copy-edges tests); the dataentry test App builders; `just seqtrace-compare`
(call flows, complements but does not replace wire parity).

**Technical Approach:** (revised after design review, RR list on the ticket)
- **Production wiring.** The corpus app is built through `NewApp` from `appbuildtest` services, with `SelectCommandAuthorizer` as the server does, not through `newAppFromParts`. The store goes in through `appbuildtest.WithStore` so every reader (App.Services().Store, entitymanager, tracer, ACL membership walk) is counted.
- **Instruments.** `storetest.Counting` with `Breadth` and `BodyWatch` stacked; record calls by method, breadth and body loads. Fix `Counting` first: forward `SoftDelete()` (so delete takes the production path) and share its mutex through a pointer (Tx race). Version reads are counted through a counting `VersionService` wrapper or recorded as not counted.
- **ACL modes.** App variants: Declarative (admin, editor, viewer, outsider, plus an `unmatched_principal: reject` principal with and without a JWT gate), NopACL, ReadOnlyACL, nil ACL. These are the arms of the five type switches PR 2 moves.
- **Fixture.** memstore, built in code with fixed ids: a faced type; a row-gated type; a `visible:` field whose `when:` takes both arms, one `when:` using `related()`; relations crossing into hidden rows; one attachment (stubbed id); two versions of one entity with `app.versions` wired; comments; a pile; a document incl. an elevated one; a command; a feed. At least 3 visible rows per listed type so N+1 changes the count.
- **Cases.** Real router via `httptest`. Fresh app per case, or a warm-up request then `Reset()` before the measured one, so `-run` on one case passes. Endpoints: list (incl. filter and sort on a hidden field, `?world=`, `_position`), detail, relations, `_relation_history`, `_sidepanel`, `_copies`, views, nav/menu, dashboard, gantt, search, export, attachments, history, `_comments`, `_piles`, `_documents`, `_commands`, `_analyze`, `_next_action`, `_action/`, `_actions`, `_feeds`/caldav, and writes (create, update, relation create, delete, bare-id ambiguity) each on a fresh app. A guard fails when a route from the router walk (`router_walk_test.go`) has no case or explicit exemption.
- **Recording.** One file per case: `{status, headers: [X-Total-Count, Link, ETag, Location, Allow, WWW-Authenticate], reads, body}`. Inputs are fixed instead of normalized; only `UpdatedAt` and audit times are replaced, by value (so a created id is replaced everywhere it appears). ETags stay: they hash served edges and face.
- **Faults.** `failclosed_test.go` stays the guard for gate faults; the corpus records healthy-path behaviour only.
- **Codemod.** Free functions `setTestACL(t, app, a)` / `setTestResolver(t, app, r)` (not methods: App's method set is capped), documented "call before NewRouter". Rewrite all 366 `app.acl =` / `app.fieldResolver =` sites, including `d.app.*` receivers; struct literals that set `fieldResolver` and the wiring assignment in `test_helpers_test.go` stay raw and are listed in the helper doc.
- **Rename.** Parameter and doc in acl/policy.go, router.go.

Alternatives rejected: an external recorder over a running server (seqtrace
demo): needs postgres and a build, too slow for every PR; snapshotting only
status codes: misses field redaction and edge-filter drift, the exact risks of
the refactor.

**Files to modify:** internal/store/storetest/counting.go (SoftDelete
forwarding, mutex); internal/acl/policy.go; internal/dataentry/router.go; new
internal/dataentry/golden_parity_test.go, testhelpers_acl_test.go,
testdata/golden/**; about 40 internal/dataentry/*_test.go files (codemod).

## Security

- [x] Security implications considered
- [x] Input validation planned
- [x] Auth/authz requirements identified

**Security Considerations:** No production behaviour change. The corpus is a
security asset: it pins row gating, field redaction and edge filtering per
principal, so later PRs cannot drift silently. Known gaps (BUG-2TPO2F,
BUG-HZ4FHZ) are recorded as they are today and marked in the case names.

## Test Plan

- [x] Unit test scenarios defined
- [x] Integration test scenarios defined
- [x] Edge cases documented

**Test Scenarios:** As acceptance criteria 1-6.

**Negative Tests:** Hidden entity read returns the same 404 body as a missing
one; hidden field absent from detail and list; edge to a hidden entity absent
from relations; outsider gets empty lists, not errors; a write by viewer is
refused with the same body as today.

## Risk Assessment

- [x] Risks identified
- [x] Mitigations planned
- [x] Effort estimated

**Risks:**
- Flaky golden files from map order or time: normalize by key, run `-count=20` and `-race` before committing.
- Codemod changes test meaning (an assignment on a different app, or after the router captured the old value): helpers do exactly the assignment; full suite must pass with zero test edits beyond the rewrite.
- Corpus too narrow to catch drift: case list reviewed in design review against the six access PRs.

**Effort:** L (upper end; the endpoint guard and ACL-mode variants are most of
it)
