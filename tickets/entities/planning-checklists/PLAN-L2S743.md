---
id: PLAN-L2S743
type: planning-checklist
title: 'Planning: Realtime 3/5: entity-change triggers and auto lifecycle'
started: "2026-10-17"
completed: "2026-10-17"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: entity-change triggers (`enter`, `leave`, `change`) evaluated on
a fixed 500ms tick from store events, with one header batch per type and a
sharded `state.KV` seen-set that stores fingerprints. System posts by
`system:realtime` with `triggered_by`. Lifecycle `auto.when` (close and archive)
on the same engine. A cross-process round lock for deduplication. A periodic
reconcile for lost events and `today()` rules. Load rules: no ephemeral trigger
posts; literal props must pass the message type's `when:`; family-anchored faced
types under face grants are refused. Wire fields `author.system` and
`triggered_by`. Out: `auto.after`, user lifecycle ops, retention, actions and
promote (T4, TKT-964SIS). Cross-node live wakes and leader election (phase 5).
Triggers on related entities (relation hops, follow-up). `props` substitution
(refused by T1). Operator-configurable reconcile cadence. Running triggers in
the CLI or stdio MCP.

**Acceptance Criteria:**
1. enter posts exactly one system message with literal props into the entity's own channel. Test: `TestTriggers_EnterLeaveChange`.
2. leave posts on stop-matching; a delete posts nothing. Test: same table.
3. change posts only when a property the rule reads changes. Tests: `TestTriggers_ChangeIgnoresUnreadFields`.
4. A new or edited rule baselines silently. Tests: `TestTriggers_NewRuleBaselinesSilently`, `TestTriggers_EditedRuleRebaselines`.
5. Changes inside one tick coalesce; triggers run before auto rules. Tests: `TestTriggers_CoalescesWithinTick`, `TestTriggers_RuleOrder`.
6. One header batch per type per round and at most 16 KV reads per rule, flat at 10 and 50 changed entities. Tests: `TestTriggers_Budget10vs50`, `TestTriggersBudget_Store10vs50`.
7. No Lua or automation runs. Test: `TestTriggers_NoScriptEngine`.
8. Hidden-field rules fail at load. Test: `TestLoad_TriggerHiddenField` (extended).
9. Degraded or off means idle; reconcile catches up on recovery. Tests: `TestTriggers_DegradedAndOffAreIdle`, `TestTriggers_RecoveryReconciles`.
10. Two engines on shared state produce one post. Tests: `TestTriggers_TwoEnginesOnePost`, `TestTriggers_ReconcileReReadsUnderLock`.
11. Dropped events and `today()` rules are caught by reconcile. Tests: `TestTriggers_ReconcileCatchesDroppedEvent`, `TestTriggers_TodayRuleViaReconcile`.
12. A rename posts no spurious enter. Test: `TestSeenHooks_RenameMovesEntries`.
13. A closed or archived target channel is skipped. Test: `TestSystemPost_ClosedChannelSkips`.
14. auto.when closes on enter, honours `from`, respects a reopen, audits `triggered_by`. Tests: `TestAuto_*`, `channelstest LifecycleTriggeredBy`.
15. New load rules refuse with file, key and reason. Tests: `TestLoad_TriggerEphemeralRefused`, `TestLoad_TriggerPropsFailWhen`, `TestLoad_FamilyFacedUnderFaceGrants`.
16. Plimsoll and arch-lint pass. Test: `just lint`.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (effort m; prior art is in-repo and phase3-plan section 3
covers the design)

**Existing Solutions:**
- Libraries: none needed. Predicate evaluation, locks and KV already exist in-repo.
- `internal/desktopnotify/tracker.go`: state.KV seen-set with a silent first evaluation; extended here with fingerprints (for `change`), shards and an index.
- `internal/changefeed/changefeed.go`: carries types only, so rejected as the source; raw `store.Watcher` events carry ids (`internal/store/store.go` Event).
- `internal/store/pgstore/sweep.go` (try-lock reconcile) and `internal/lock` (`lock.For`, BackendLocker): the cross-process round lock. `internal/filelock`: the fs and sqlite tier.
- `internal/visibility/allowall.go`: the AllowAllReader capability for system reads. `principal.UserScheduler` and `UserAutomation`: the pattern for `system:realtime`.
- `internal/autocascade/runner.go`: the `triggered_by` attribution convention.
- `internal/appbuild/aliasfanout.go`, `channels.go`: alias subscriber wiring for the rename hook.
- `internal/realtime/service.go` (`record`, `Post`), `local.go` (`mark`): the system post path and live marking.
- `internal/predicatefns/evaluator.go` (`Matches`): rule evaluation over header props.
- rela concepts: DEC-ZBI39P (AllowAllReader), TKT-WZYWM9 (pg NOTIFY delivers cross-process events), docs/postgres-backend.md (several nodes).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** See .ignored/t5-plan.md sections 0 to 4.
- appbuild forwards raw store entity events to `realtime.Triggers.Note`, which keeps a bounded pending set.
- Every 500ms a round does the following:
  1. loads one config snapshot and one clock reading;
  2. takes the cross-process round lock (pg keyed lock, `filelock` on fs and sqlite);
  3. reads the changed rows with one header batch per type through AllowAllReader;
  4. reads the touched seen shards from `state.KV`;
  5. evaluates the allowlisted rule programs;
  6. diffs against the stored fingerprints to get enter, leave and change;
  7. posts system messages (author `system:realtime`, `triggered_by: trigger:<name>`) through an unexported `systemPost`; auto rules move channels through the shared `applyTransition` (T5b);
  8. saves the shards and releases the lock.
- A reconcile every 15m (and at start) scans each trigger type and re-reads the differing ids under the lock.
- New rules baseline silently. Degraded config means idle.

Alternatives rejected:
- entitymanager post-commit hook: sees only this process's Manager writes, misses external, import and `today()` changes, and adds work to every write path.
- automations or Lua: forbidden by the ticket.
- jobs.Queue: not an external effect, and IdempotencyKey deduplicates only while pending.
- changefeed.Bus: types only.
- Deferring deduplication to phase 5: duplicate posts on several nodes.

Dependencies: `predicate`, `predicatefns`, `channels`, `realtimecfg`,
`realtimelimits`, `principal`; and, through appbuild adapters, `store`, `state`,
`visibility`, `lock`, `filelock`.

**Files to modify:**
- New: `internal/realtime/trigger.go`, `trigger_rules.go`, `seen.go`, `system.go`, `source.go`, and tests; `internal/appbuild/realtime_triggers.go` and test.
- Changed: `internal/realtime/service.go` (`recordType`), `render.go`; `internal/realtimelimits/limits.go`; `internal/realtimecfg/compiled.go`, `programs.go`, `validate.go`, `load_test.go`; `internal/acl` (`FacesRestricted`, if D5); `internal/principal/principal.go`; `internal/channels/channels.go`, `filechannels`, `sqlchannels`, `channelstest` (T5b); `internal/appbuild/appbuild.go`; `cmd/rela-server/serve.go`; `cmd/rela-desktop/main.go`; `internal/apiwire/v1/channels.go`; `internal/openapi/channels.go`; `docs/realtime.md`; `docs/postgres-backend.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- realtime.yaml triggers and `auto.when` (operator config):
  - rules are allowlisted at load: no traversal; only `contains`, `len`, `today`, `days_between`, `date_add`; only `entity.<prop>`; no `visible:`-restricted field;
  - props are literal only, with no `{{}}`; they must validate against the message type and pass its `when:` (new);
  - an ephemeral `post.type` is refused (new);
  - family-anchored faced types under face grants are refused (new, D5);
  - every violation is a load error naming file, key and reason.
- Entity rows (store data): bound only through `predicatefns.EntityRecord` over declared properties. Evaluation errors count as no match and change nothing.
- Store events: used only as "which ids to re-read"; never trusted for values.
- Seen-set KV values: decoded defensively; an undecodable shard counts as unknown and baselines silently, with no burst. Keys are built from the rule id (name regex plus hex) and a shard number, so `state.ValidateKey` holds.

**Security-Sensitive Operations:**
- System posting bypasses role authorization. It is unexported (`systemPost`, `systemTransition`), reachable only from the in-package engine built at wiring, and attributed to the reserved `system:realtime` principal, which external identities cannot assert.
- AllowAllReader is handed out only at the wiring site, for the trigger engine. Its outputs never reach a reader, except as the literal operator text posted into the anchor's own channel. That channel's readers can already read the anchor (`Read.RequireAnchorRead`).
- The seen-set stores truncated SHA-256 fingerprints of rule field values, never the raw values.
- The one-bit face leak on family channels is closed by D5.
- Logs name the rule and entity id only, never property values.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See the table in .ignored/t5-plan.md section 8. Every
criterion maps to named tests:
- unit tests in `internal/realtime` with fake EntitySource, KV, lock and clock;
- integration through the appbuild adapter with `storetest.Counting` on the memory store and a real `filelock` in a temp dir;
- channel-store conformance (`channelstest`) on fs, sqlite and schema-pinned postgres for `TriggeredBy`;
- load tests in `realtimecfg`.

**Edge Cases:**
- No realtime.yaml, a degraded snapshot, or a snapshot pointer change mid-flight (the round uses its captured snapshot).
- An entity with no channel yet (implicit create on first post); a closed or archived channel (skip); a cardinality-one channel never written (auto skips).
- Faced types: `FacePrime` versus `FaceEach`; deleting one face while another remains.
- Enter and leave inside one tick (coalesced); more than 64 posts in a round (deferred, entries not updated); pending past 8192 ids (overflow, then full reconcile of the type).
- A rename racing a round; a lock timeout in the rename hook; soft delete then restore (enter again, accepted); id reuse within the grace period.
- A rule evaluation error (no match, entry unchanged, no leave); an undecodable shard (baseline); a KV write failure after the post (at-least-once repeat on the next round).
- A `today()` rule crossing midnight with no write (reconcile).
- Two processes receiving the same event (one post).

**Negative Tests:**
- Load refuses: ephemeral trigger `post.type`; literal props failing the `when:`; hidden-field rules; traversal or disallowed functions (existing); family-anchored faced types under face grants.
- An HTTP or MCP post cannot claim `system:*` (reserved prefix; existing principal tests plus a handler test asserting `author.system` is false for a user post).
- With no Lua engine wired, a trigger round still works and calls no script runner.
- A KV or lock error makes the round fail closed: no posts, pending kept, error logged.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
1. Several processes: correctness rests on the round lock plus the shared KV; reads are repeated per node; a long reconcile holds the lock. Mitigation: re-read only candidates under the lock. Leader election in phase 5.
2. A rename race or lock timeout gives one spurious enter. Mitigation: the hook takes the lock with a 2s wait; logged.
3. Restore or id reuse posts enter again. Accepted and documented.
4. Lossy events and sqlite with no cross-process events. Mitigation: the 15m reconcile.
5. Seen-set rewrite cost. Mitigation: 16 shards; a round touches only the shards it needs.
6. A bulk import creates many posts and channels; channels block deletes under `on_entity_delete: block`. Mitigation: per-round cap; documented.
7. Face leak on family channels. Mitigation: D5 load refusal.
8. Orphan channel when a delete races an Append. Unreachable, and tombstoned on reuse (T3 residual).
9. Merge conflicts with T4 (`channels.Transition`, `render.go`, `realtimecfg`). Mitigation: T5b goes on top of T4; one owner for `applyTransition`.

Effort: m (T5a about 2/3, T5b about 1/3).

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] ~~docs/metamodel.md - New metamodel features~~ (N/A: realtime config is documented in docs/realtime.md)
- [x] ~~docs/cli-reference.md - New/changed commands~~ (N/A: no CLI change in T5)
- [x] ~~docs/data-entry.md - UI changes~~ (N/A: no SPA change; SPA is T7)
- [x] CLAUDE.md - New patterns or conventions (realtime background work runs only in the leader worker; system posts carry triggered_by)
- [x] ~~README.md - Project-level changes~~ (N/A: no project-level change)
- [x] ~~N/A - Internal change~~ (N/A: user-facing docs are updated)
- [x] docs/realtime.md: trigger semantics (enter, leave, change, coalescing, silent baseline), delivery (at-least-once, reconcile cadence, `today()` latency), hosts, `system:realtime` and `triggered_by`, auto.when edge semantics, new load rules, bulk and `block` interaction.
- [x] docs/postgres-backend.md: several-node trigger behaviour (round lock, one post, repeated reads).
- [x] docs/acl-security.md: AllowAllReader for the trigger engine; family/face rule (D5).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-21BQ79, RR-H0VKN4, RR-BEX7AE, RR-B0VLJB,
RR-0PL1RV, RR-EHNQ6Y, RR-MHD4DH, RR-ACVH7G, RR-2SKCFW, RR-RNYIVB, RR-YM39IT,
RR-E269SG, RR-DAHMVU, RR-JDOE33, RR-G9DAPS, RR-EFVRNV, RR-WJHM8I, RR-0P2850,
RR-3OQOSI, RR-JXEXXJ, RR-M3PYE4 (all addressed in the plan; decisions from
Jeroen recorded in the plan's section 12)
