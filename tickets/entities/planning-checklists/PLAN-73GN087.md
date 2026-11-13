---
id: PLAN-73GN087
type: planning-checklist
title: 'Planning: Remote sync status, conflict list and guides'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: a status page per remote and a conflict list with resolve, dismiss, send
again and link in the data-entry app; `rela remote link`; audit ops
remote-resolve, remote-dismiss and remote-link on CLI and web; a structured
cause on refused and uncertain entries; lock-only runs that leave the task
schedule alone; two docs-project guides replacing docs/remote-sync.md;
RR-KUA77YM (relation push tests).

Out: a web-triggered sync (owner Q2); deletes (TKT-CVT9W1K); RR-ENVA2RY,
RR-BGUZIUA, RR-UKLW3VN, RR-68KJOJ2 and RR-XGCZSM5 stay deferred.

Owner decisions (2026-11-11): Q1 permission `remote:sync` like git:sync, entries
gated per row and field, taking the remote value also needs edit rights on the
local field; Q2 no web sync, page shows the next run; Q3 send again with a
warning plus link to an existing remote row; Q4 run health from scheduler state,
gated per-viewer counts, no error or refusal text, credential kind only; Q5
audit ops without values; Q6 one PR.

**Acceptance Criteria:**
1. A principal with remote:sync sees each remote's config summary, run health and gated counts on /remotes. Test: handler test with two principals; E2E.
2. Without remote:sync every route answers 403 permission_required naming remote:sync. Test: handler test.
3. The conflict list leaves out entries for hidden rows and hidden fields, and counts reflect only shown entries. Test: handler test with an acl.yaml that hides a row and a visible: field.
4. Resolve/dismiss/link change only the ledger and list, under the run lock, without stamping LastRun, and write an audit record. Test: scheduler and handler tests; audit sink assertions.
5. Take remote is refused (403) when the caller cannot edit the local field, and `_actions` agrees. Test: affordance/gate agreement test.
6. A hidden or missing entry answers the uniform 404 on every write route. Test: handler test.
7. Link records the remote address for an uncertain create; the next run treats it as linked and sets the ref. Test: remotesync test with the fake remote.
8. Superseded entries under retention 0 are marked as having no stored local value. Test: handler test.
9. Guides are generated into docs/. Test: just docs; check output.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: approach settled by DEC-2NFLAHB and the owner's answers)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal operator UI over existing state)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: the git `_conflicts` view is the model)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- git `_conflicts` view: `permitsGitSync` (handlers_git.go), `conflictsReadable` batch gate (conflict_gate.go), ConflictsView.vue.
- Flow routes: consumer-side `FlowService` installed by `SetFlows` (flows_handler.go), wired in dataentrywire.
- Field gating: `primeVerdicts` + `FieldVerdicts.IsVisible/IsWritable` (affordances.go).
- Non-entity audit ops: OpTokenSet, OpHistoryReveal.
- scheduler.ResolveRemote/DismissRemote and startManualRun as the run lock.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- acl: built-in permission `remote:sync`.
- schedulerstate: `Outcome.LockOnly` leaves the task state alone, so a dismiss, purge, web resolve or link does not count as a successful pull.
- remotesync: `Conflict.Cause` (structured, set for refused and uncertain entries; old entries derive a status from the stored detail); `Link(ctx, kv, m, local, address)` for an uncertain create (ledger row linked, adopted, base = the digests it tried to send, hash zero); `LoadLedgerSummary` (linked local ids per local type and relation keys) for gated counts.
- scheduler: `RemoteStatuses` (config summary + TaskState), `ResolveRemoteOnly`, `LinkRemote` using a lock-only run; `DismissRemote` and the purge become lock-only.
- audit: OpRemoteResolve, OpRemoteDismiss, OpRemoteLink with a helper; recorded by CLI and web.
- dataentry: `RemoteSyncService` interface with dataentry-local types, `SetRemotes`; routes GET `/api/v1/_remotes`, GET `/_remotes/{name}/conflicts`, POST `/_remotes/{name}/conflicts/_resolve|_dismiss|_link`. Gate: remote:sync via the git:sync predicate shape; rows in one batch per type, relation endpoints in one batch, entities loaded once per type for field verdicts; one function decides each entry's `_actions` and the POST gate.
- dataentrywire: adapter from scheduler/remotesync to the dataentry types.
- CLI: `rela remote link`; audit records on resolve/dismiss/link.
- Frontend: RemotesView, RemoteConflictsView, footer link, api client, fakeApi + contract, unit tests, E2E.
- Docs: GUIDE-remote-sync, GUIDE-remote-sync-two-way; remove docs/remote-sync.md; API reference, OpenAPI, GUIDE-acl-security threat model, CLI reference.

Alternatives rejected: persisting the run report (counts cannot be gated);
serving refusal text (may carry remote secrets); gating in appbuild (dataentry
already owns the gates the git view uses).

**Files to modify:** internal/acl/policy.go,
internal/schedulerstate/schedulerstate.go,
internal/remotesync/{conflicts,push,relsync,ledger}.go,
internal/scheduler/remote_conflicts.go, internal/audit/audit.go,
internal/cli/remote_*.go, internal/dataentry/remotes_handler.go (new),
api_v1.go, openapi, internal/dataentrywire/wire.go, .go-arch-lint.yml,
frontend/src/{api,views,router,components,test/fakeApi}, e2e/tests,
docs-project/entities/guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Remote name in the path: `principal.ValidIntegrationName`, else 404; unknown remote 404.
- local, field, take, address in POST bodies (body limit): take enum; field must name an entry; address parsed as `type/id` of the mapped source type with `entity.ValidateID`.
- Entries from state.KV: treated as stored data; remote values served only for visible fields.

**Security-Sensitive Operations:**
- Reads: remote:sync + row gate + field visibility; counts after gating; nothing cached.
- Writes: remote:sync + read of the row + (take remote) field edit right; run lock; audit without values.
- No credential source names or values, no error text, no refusal text on the wire.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the acceptance criteria; plus a storetest.Counting
budget test at 10 and 50 entries, frontend unit tests per view, the
fake-vs-server contract test, and an E2E spec that seeds a conflict list and
drives resolve and dismiss.

**Edge Cases:** empty list; remote never run; remote busy (409); entry for a
deleted local row; relation entries with one hidden endpoint; superseded with
retention 0; omitted values; link to an address already linked; link on a
non-uncertain entry.

**Negative Tests:** no permission (403); hidden row (404); bad take (400); bad
address (400); unknown remote (404).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Size (about 5k lines): one PR as decided; keep handlers small.
- Archplan fan-in on remotesync: only dataentrywire and scheduler import it; dataentry uses its own types.
- Lock-only change touches both scheduler-state backends: covered by schedulerstatetest.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-remote-sync and GUIDE-remote-sync-two-way (new),
GUIDE-data-entry, GUIDE-cli-reference, GUIDE-acl-security, API reference,
OpenAPI; docs/remote-sync.md removed.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-105G1ZB (exact-entry resolve), RR-87P2GGM
(relation meta gating), RR-K008G0E (row-level take remote needs every field
writable), RR-83TRVEA (faces from the gate), RR-BC4P60I (gate then page). All
folded into the approach.
