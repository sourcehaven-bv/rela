---
id: PLAN-UKLXH9
type: planning-checklist
title: 'Planning: Realtime 3/4: channel lifecycle, actions and promote-to-graph'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In:
- declared lifecycle ops (close/reopen/archive/delete) with gate, `from`, `condition`, `denied`, and delete `retention`;
- auto.after in a jobs.Queue sweep;
- message actions with gate;
- promote-to-graph through entitymanager.Manager (CreateEntity + CreateRelation as the caller, triggered_by), with at-most-once plus visible-pending semantics and a moderator release;
- `on_entity_delete: delete` made real (tombstone before Tx, purge after commit);
- purge of detached and orphaned channels;
- clearing type records of removed channel types;
- HTTP `_lifecycle` and `_action` routes, error types, `_actions` affordances, OpenAPI, docs.

Out:
- entity-change triggers and `auto.when` (T5, TKT-R0Z2G4);
- MCP tools and CLI (T6, TKT-76G6M7);
- SPA controls;
- configurable detached retention;
- a `by: human` rule (superseded: agents are ordinary principals);
- cross-process live delivery beyond T3's head polling.

**Acceptance Criteria:**
1. Undeclared lifecycle op: not offered, 403 `lifecycle_not_offered` naming the rule. Test: TestLifecycle_NotDeclared.
2. Gate failure: 403 `lifecycle_denied` with the `denied:` text. Test: TestLifecycle_GateDeniedUsesDeniedText.
3. `from` honoured: 409 `lifecycle_wrong_state`, not offered. Test: TestLifecycle_FromWrongState.
4. `condition` honoured on the redacted record; degraded mode denies. Test: TestLifecycle_Condition_TrueFalseRedactedDegraded.
5. Retention blocks delete until closed_at+period (409 `retention_active`); a delete afterwards keeps a deletion record. Test: TestLifecycle_DeleteRetention.
6. Transitions bump the rev, audit, wake subscribers; a concurrent op gives one 409. Test: TestLifecycle_TransitionsAuditAndMark, TestLifecycle_ConcurrentCAS.
7. The lifecycle affordance and the POST agree. Test: TestLifecycle_AffordanceAndGateAgree.
8. auto.after closes and archives due channels (calendar-exact), not in degraded mode. Test: TestSweep_AutoAfter_FixedAndCalendar, TestSweep_SkipsAutoWhenDegraded.
9. `block` refuses an entity delete. Test: TestChannelGuard_BlockRefuses.
10. `delete` policy: channels unreachable at once, erased after commit, restored on Tx failure, boot warning gone. Test: TestChannelGuard_DeletePolicy_*.
11. Detached and orphaned channels purged after 30d, never before retention; orphans only after grace. Test: TestSweep_PurgeHonoursPolicyAndRetention, TestSweep_OrphanGrace.
12. Type records of removed, empty types cleared; kept when in use or degraded. Test: TestSweep_TypeRecordCleared_KeptWhenInUseOrDegraded.
13. Undeclared or denied action: 403 `action_not_offered` / `action_denied`. Test: TestAction_NotDeclared, TestAction_GateDenied.
14. A double accept creates exactly one entity. Test: TestAction_DoubleAcceptOneEntity.
15. The action affordance and the POST agree. Test: TestAction_AffordanceAndGateAgree.
16. Promote under create ACL / field gate: 403/422 and the action reverts. Test: TestPromote_ACLDeniedReverts, TestPromote_FieldGateReverts.
17. Promote appears in the graph audit with `triggered_by: channel:…#seq/action`. Test: TestPromote_TriggeredByInAudit.
18. Promoted entity related to the anchor; a relate failure warns, entity kept. Test: TestPromote_RelatesToAnchor, TestPromote_RelateFailureWarns.
19. Mapping reads message props only; face passed. Test: TestPromoteProps_MessagePropsOnly_Face.
20. An agent principal holding the gate can act. Test: TestAction_AgentPrincipal.
21. Stale pending reported, never auto-reverted; moderator release after 15m. Test: TestSweep_StalePendingReportedNotReverted, TestRelease_GateAndAge.
22. OpenAPI lists only declared ops and actions and follows reload. Test: TestOpenAPI_ChannelLifecycleAndActionPaths.
23. Store conformance on fs, sqlite and pg (schema-pinned DSN). Test: channelstest Tombstones/Purge/DropTypeRecord/ReleasedAudit.
24. Read budgets flat at 10 and 50. Test: TestChannelMessages_ActionsBudget, TestSweep_Budget.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the design is fixed by phase3-plan.md decisions and the T3 plan; research consisted of reading the existing realtimecfg, channels, realtime, entitymanager and jobs code)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (.ignored/phase3-plan.md and .ignored/t4-plan.md)

**Existing Solutions:**
- Libraries: none needed. Scheduling uses the in-tree jobs.Queue (fs ephemeral, pg durable neoq fork); periods use realtimecfg.Period.AddTo.
- Codebase patterns reused:
  - internal/realtimecfg already compiles Lifecycle/LifecycleOp/Gate/Auto/Retention/Action/Promote and validates promote maps, faces and relate tuples at load;
  - internal/channels Store has SetLifecycle and SetAction CAS, Delete with a deletion audit, ListByLifecycle, StalePending, Tombstone/Untombstone with handles;
  - internal/realtime `open`/`gate`/`gateDenied`/`requireOpen`/affordances (the T3 authz algorithm);
  - internal/appbuild/softdeletegc.go (recurring job with a fixed IdempotencyKey);
  - internal/appbuild/channelguard.go (tombstone + undo before the entity Tx);
  - internal/dataentry detailActionCheck.Allows plus TestDetailAction_AffordanceAndGateAgree (one function for affordance and gate);
  - internal/dataentry/caldav_write.go linkToDriver (create-then-relate compensation precedent, deliberately not copied: see D2);
  - Manager.CreateEntity/CreateRelation with audit.WithTriggeredBy.
- Reference implementations: the outbox/"pending then confirm" pattern (Stripe-style idempotent actions), and the Slack/Mattermost interactive-message one-shot actions; exactly-once is limited by the absence of client-chosen ids, hence at-most-once plus visible pending.
- rela concepts: DEC-HWZHA write-with-warnings; "configuration is not secret"; TKT-FIO205 (channels out of the graph audit); RR-H368O2 (tombstone purge deferred to T4).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Three stacked PRs; full detail in .ignored/t4-plan.md.

- **T4a:**
  - Store gains `Tombstones(before)`, `Purge(handle, by, at)`, `DropTypeRecord(type)` and a `released` audit on pending→none, implemented in filechannels and sqlchannels (no migration expected; pg 0029 / sqlite v21 only if an EXPLAIN demands an index).
  - realtime.Sweeper: RunLifecycle every 1m (auto.after via ListByLifecycle + SetLifecycle CAS; stale-pending report) and RunRetention every 1h (purge by policy and retention, orphan detach after a 1h grace, type-record clearing), run by appbuild/channelsweep.go on jobs.Queue with fixed IdempotencyKeys.
  - channelguard `delete` policy = tombstone before the Tx (undoable), purge after commit via a post-commit enqueued retention run.
- **T4b:**
  - Service.Lifecycle runs T3 steps 1–5, then declared → caller → gate → from → condition (redacted record; degraded denies) → retention → rate → CAS / Store.Delete → mark.
  - `_actions.lifecycle` comes from the same lifecycleCheck.
  - Adds the POST `_lifecycle/{op}` route.
- **T4c:**
  - Service.RunAction runs T3 steps 1–5, then message checks → declared action (or implicit `promote`) → gate → open → status → rate → SetAction none→pending CAS → Promoter (appbuild over Manager, as the caller, triggered_by `channel:<key>#<seq>/<action>`) → done, or revert to none on failure.
  - ReleaseAction is for moderators after 15m.
  - Per-message `_actions`.
- Alternatives rejected:
  - auto-revert of stale pending (duplicates entities);
  - compensating delete on relate failure (can itself fail, and destroys a valid entity);
  - erasing channels before the entity Tx (cannot be undone);
  - a new entitymanager post-commit hook (not needed; the post-commit line already exists);
  - auto-promote on post (needs a second action slot and makes every post a graph write).
- Dependencies:
  - realtime may import only channels, realtimecfg, realtimelimits, entity, principal, predicate, predicatefns and metamodel, so Promoter and AnchorExistence are consumer-side ports implemented in appbuild;
  - jobs.Queue;
  - entitymanager.Manager (no new methods; at the plimsoll cap);
  - acl.ForbiddenError for error mapping.

**Files to modify:**
- internal/channels/{store.go, channels.go, tombstone.go}
- internal/channels/filechannels/{ops.go, file.go}
- internal/channels/sqlchannels/sql.go
- internal/channels/channelstest/channelstest.go
- internal/realtime/{sweep.go (new), lifecycle.go (new), actions.go (new), promote.go (new), authz.go, service.go, ops.go, render.go, errors.go} + tests
- internal/realtimecfg/validate.go (reserve `promote`)
- internal/realtimelimits/limits.go
- internal/appbuild/{channelsweep.go (new), realtime_promote.go (new), realtime_service.go, channelguard.go, channels.go, appbuild.go}
- internal/dataentry/{channels_handler.go, channels_errors.go, channels_wire.go}
- internal/apiwire/v1/channels.go
- internal/openapi/channels.go
- docs/realtime.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- URL path, op name: allowlisted to the four ops by the router (unknown gives 404) and to declared ops by the service (403).
- URL path, action name: allowlisted to the message type's declared actions plus the reserved `promote` (403 otherwise).
- URL path, seq: a positive int64 parsed by the existing T3 parser (400).
- URL path, entity id/face: T3 resolveAnchor through the gated reader (uniform 404, 422 face_required).
- Request body: empty or `{}`; anything else is 400 (unknown fields refused, as in T3).
- realtime.yaml: already validated at load (condition allowlist: no traversal, functions limited to contains/date_add/days_between/len/today, no visible-restricted fields; promote map/face/relate checked against the metamodel).
- Promote values: copied from stored, already validated message props, then re-validated by Manager (field gate, metamodel validation; 422 on failure).

**Security-Sensitive Operations:**
- Graph writes (promote):
  - only through Manager as the caller, so create ACL, field gate and audit apply;
  - never elevated;
  - the ceiling is checked before pending, so ReadOnlyACL refuses early.
- Conditions: evaluated on the redacted anchor record only, so no oracle on hidden fields. Degraded mode denies.
- Affordances: hints only; every POST re-authorizes through the same function (agree-tests).
- Erasure (lifecycle delete, purge):
  - gated by the declared permission and retention;
  - purge runs only after the entity Tx commits, honours retention, and leaves a deletion record.
- Sweep: acts as `system:realtime` and only applies config-declared automation (auto.after) and retention. Its ports are unfiltered by ACL by design (existence check only, no content exposure).
- Error messages:
  - name config rules and `denied:` text (config is not secret);
  - never reveal anchor existence beyond T3's uniform 404;
  - Manager errors map to a fixed problem type; the ACL rule name is fine to show.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1–AC24 map one-to-one to the tests named under Acceptance Criteria (full table in .ignored/t4-plan.md §4).
- Unit: realtime service tests over filechannels with fake Access, Anchors and Promoter, and fixed clocks.
- Integration:
  - appbuild tests with a real Manager, declarative ACL and an audit capture (promote, guard);
  - dataentry HTTP tests (routes, status codes, problem types, `_actions`);
  - channelstest conformance on fs, sqlite and pg (schema-pinned DSN);
  - an OpenAPI generator test;
  - budget tests in the storetest.Counting style.

**Edge Cases:**
- Retention exactly at the boundary (now == until is allowed); `1mo` from Jan 31; `1y` from Feb 29.
- Archive of an open channel also sets ClosedAt (retention counts from it).
- A `one` channel never written (404). A message removed between page and action (409). A message type removed from config after posting (action not offered).
- Two concurrent accepts; accept racing a release; a sweep racing a manual close (CAS conflict skipped).
- A purge of a batch already untombstoned or purged (ErrNotFound means no-op). Two nodes purging the same batch.
- An orphan anchor reappearing within the grace period (forgotten). A process restart (grace restarts).
- Degraded config: auto, conditions and type-record clearing all suspended.
- Promote of a faced type (face passed). A manual-ID type collision (409). Optional mapped prop absent (omitted). A ref prop mapped (id string).
- A ReadOnlyACL process (403 before pending).

**Negative Tests:**
- Undeclared op or action: 403. Unknown op path: 404. Non-empty body: 400. Bad seq: 400.
- Gate denied: 403 with `denied`. Condition false: 403. Wrong state: 409. Retention active: 409. Channel not open (action): 409.
- Action done: 409 `action_taken`. Pending: 409 `action_pending`. Release too early: 409. Release by a non-moderator: 403.
- Promote: ACL denied gives 403 and a revert; validation gives 422 and a revert; already exists gives 409 and a revert; relate failure gives 200 with a warning.
- Entity delete with `block`: 409. Entity Tx failure with `delete`: channels restored.
- Rate limit: 429 with Retry-After.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
1. **Crash after promote commit, before done:** an orphaned pending. Mitigated by the detached-context retry of done, the stale-pending report, the moderator release and triggered_by in graph audit; documented as at-most-once (never auto-revert).
2. **Irreversible erasure under the delete policy:** mitigated by tombstone before the Tx, purge only after commit in a job, retention honoured in purge, and an idempotent purge. Verify that soft delete does not tombstone.
3. **Condition and affordance oracles, and affordance/gate drift:** mitigated by redacted-record evaluation, degraded denial, single decision functions and agree-tests.
4. **Sweep cost and multi-node duplicates:** mitigated by the indexed ListByLifecycle, the hourly O(n) retention pass, IdempotencyKey, CAS writes and budget tests.
5. **Orphan false positives:** mitigated by the grace period and the reversible tombstone with 30d retention.
6. **Relate failure leaves an unrelated entity:** accepted with a warning; the message keeps the promoted link.
7. **Calendar period arithmetic:** Period.AddTo only, with boundary tests.

Effort: l (three PRs: T4a store/sweep/guard, T4b lifecycle, T4c
actions/promote).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: no metamodel change; realtime config is documented in docs/realtime.md, which gets lifecycle/actions/promote behaviour, HTTP routes, error types, `_actions`, delete policy, purge timing, the orphan rule and at-most-once/release)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: CLI is T6)
- [x] docs/data-entry.md - UI changes (api-reference: new `_lifecycle`/`_action` routes and `_actions` fields; no SPA change in T4)
- [x] CLAUDE.md - New patterns or conventions (internal/realtime or root note: promote only via the Promoter port over Manager; affordance and gate share one function; stale pending is never auto-reverted)
- [x] ~~README.md - Project-level changes~~ (N/A)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-XLXBNE, RR-1G1TZM, RR-O7PAIE, RR-TONEXN,
RR-1F35OK, RR-7K5H71, RR-38TDPI, RR-X0POFN, RR-0OI5LM, RR-INR7CH, RR-1W68DR,
RR-LZB28B, RR-RNF0TT, RR-JO296I, RR-1KP8CD, RR-SGVDFB, RR-M79K40, RR-8SVFHR,
RR-919QPZ, RR-POQ1KF, RR-CUBN6B, RR-V4ZTX4, RR-5WVZJN, RR-0QCXLM, RR-F45VDR,
RR-3HCJX9, RR-01O53K, RR-NOGEJJ, RR-WIO22D (all addressed in the plan; decisions
from Jeroen recorded in the plan's section 12)
