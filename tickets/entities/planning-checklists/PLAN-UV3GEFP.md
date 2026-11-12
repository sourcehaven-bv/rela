---
id: PLAN-UV3GEFP
type: planning-checklist
title: 'Planning: Two-way sync: send deletes with guards'
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

In: on a two-way remote (push: true), local deletes of linked entities and
relations are sent to the remote through its DELETE routes (new `send_deletes`,
default false), and remote deletes follow `on_gone` for rows the driver pushed
too. A delete that meets an edit on the other side follows the type's conflict
rule. Guards shared with on_gone. If-Match (and 412) on the entity and relation
DELETE routes. Conflict-list reasons and resolve for delete conflicts (CLI and
web). Guides and OpenAPI.

Out: faced types (not synced at all); one-way remotes keep today's behaviour
except the guard change below; deletes of rows the local principal cannot read
(never named); web entries for a row deleted locally (the row is gone, so the
per-row gate hides them; CLI shows them); per-type send_deletes override.

Owner decisions (2026-11-12): two per-remote settings, both off by default:
on_gone (detach | delete) follows remote deletes, on a two-way remote also for
pushed rows; send_deletes pushes local deletes. Delete vs edit follows the
conflict rule (hold default; local/remote decide; latest holds, no trusted
delete time); resolving picks delete or keep, keep restores the row on the side
that deleted it. Guards: deletes in both directions share max_gone per run and
per 24 h, emptying a type is refused, --allow-gone N confirms; over the guard
only the deletes wait and the rest of the run proceeds. DELETE honours If-Match;
412 is a conflict, 403/409 hold the delete on the conflict list; the remote's
cascade applies as for any API caller.

**Acceptance Criteria:**
1. send_deletes: true sends a local entity delete and a local relation delete to the remote; both rows are gone on the remote and the ledger forgets them. Test: dataentry two-instance e2e.
2. on_gone: delete on a two-way remote deletes a local row the driver pushed when the remote deletes it. Test: e2e.
3. A local delete of a row the remote edited since the last sync is held (hold), restores the local row (remote), or is sent (local). Test: unit per rule, e2e for hold.
4. A remote delete of a row edited locally is held (hold) / kept and recreated on the remote (local) / deleted (remote). Test: unit.
5. Guard: more deletes than max_gone (both directions together, per run and per 24 h) or emptying a type: no delete either way, the rest of the run applies, Report.Refused says why, Run returns ErrGoneGuard; --allow-gone N lets them through. Test: unit + e2e mass delete.
6. DELETE with a stale If-Match answers 412 and deletes nothing; a matching one deletes. Entity and relation. Test: dataentry handler tests.
7. Resolving a delete conflict: take local sends the delete / keeps the local row and recreates on remote; take remote restores locally / deletes locally. Test: unit through Resolve + next Run.
8. send_deletes without push fails to compile; rows dropped before send_deletes was turned on are never sent. Test: config + unit.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-G8CYB1R (feature research) covers sync; no new research:
the owner decided the policy.

**Existing Solutions:**
- on_gone guard (internal/remotesync/run.go guard/emptied/goneEntity/goneRelation) is the model for the shared guard.
- Push outcomes (push.go pushRefusedOrFailed, If-Match + 412 retry) model the delete send.
- Conflict list + Resolve (conflicts.go) model delete conflicts.
- PATCH If-Match on entities (write_handler.go:1057) and relations (relationIfMatch) model the DELETE precondition.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Config: `send_deletes` in remotestatus.Settings (status shows it); Compile
refuses it without push. Each mapping keeps a row rule = type conflict rule,
else remote rule, else hold (field_conflicts do not apply: a delete is a whole
row).

Ledger: new state `deleting` (local row deleted, delete to send) and a row field
`Resolve` (side a person chose, consumed by the next run). The sweep turns a
linked row whose local row is gone into `deleting` when send_deletes is on, else
`dropped` as today. Rows dropped before send_deletes was turned on stay dropped,
so turning it on never sends old deletes. A deleting row keeps its base digests
until the delete is sent or forgotten (RR-VE812QV); a dropped row's are cleared
as today.

Run:
- prepare: consume Resolve on held delete rows (deleting+remote: forget the row so the pull restores it; detached+remote: back to linked so the gone path deletes it without the edit check).
- pull: a `deleting` row is never fetched or recreated by the pull.
- guard: plan = gone rows to detach/delete + deleting rows to send. Over max_gone (per run, and with the 24 h window for deletes), or a type emptied on either side, and not confirmed by --allow-gone: no gone row and no send is touched; the rest applies; Refused set; Run returns ErrGoneGuard at the end. A deleting row both sides deleted is forgotten.
- gone (remote deleted), on a push remote with on_gone: delete: adopted rows are deletable; a row edited locally (mapped fields off base, relation meta likewise) follows the row rule: hold/latest = detach + entry `remote_deleted`; local = keep and recreate on the remote (entity: push creates a new remote row, ref reset; relation: ledger row forgotten so the relation push creates it); remote = delete.
- send (after the entity and relation push, relations first): GET the remote row with its ETag (404 = gone, forget), compare the mapped fields with the base; changed = remote edit. Otherwise DELETE with If-Match on the fetched ETag. 2xx/404 forget the row and count in the delete window; a 412 fetches once more; a remote edit follows the row rule (hold = entry `local_deleted` cause edited; remote = forget the row, next pull restores it; local = fetch and retry once); 400/403/409/422 = entry `local_deleted` cause refused; other = failed, retried next run. Budget counted like a push.
- forgetConflicts keeps delete entries while their ledger row is still deleting/detached.

Conflicts: reasons `local_deleted` and `remote_deleted`, row-level, Cause
edited|refused. Resolve sets Resolve on the ledger row and drops the entry;
dismiss stays for superseded only. PushCounts.Deleted; Counts.Deleted already
exists.

Server: entity DELETE and relation DELETE check If-Match against the same ETag
as GET/PATCH and answer 412 on mismatch.

Web/wire: dataentrywire and the remotes handler accept the new reasons; their
verdicts follow the existing take-local/take-remote checks.

Alternatives rejected: one `deletes` switch (owner wanted per-direction);
fetch-and-compare without If-Match (race); keeping base digests of deleted rows
to judge field-level edits (keeps digests of deleted values; the diff hash and
ETag already tell whether the remote changed).

**Files to modify:** internal/remotestatus/remotestatus.go;
internal/remotesync/{config,ledger,run,push,relsync,conflicts,report}.go plus
new deletes.go and tests;
internal/dataentry/{write_handler,softdelete_handler?}.go, remotes_handler.go,
OpenAPI spec, new tests incl. remote_delete_e2e_test.go; internal/dataentrywire;
internal/cli/remote_pull.go help; frontend remote status if it lists
settings/reasons; docs-project guides GUIDE-remote-sync and
GUIDE-remote-sync-two-way and API reference; regenerated docs/.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- remotes.yaml (operator, trusted): send_deletes is a bool, refused without push.
- Remote answers (untrusted): status codes only drive the outcome; refusal detail is sanitized and capped as for pushes; a remote id never names a local row (only the driver's ledger maps).
- If-Match header (any API caller): compared exactly to the server-computed ETag.

**Security-Sensitive Operations:**
- Local deletes go through entitymanager as the run's principal (ACL, audit); rows the principal cannot read are never deleted or named (existing readable checks).
- Sent deletes use the remote's normal DELETE route as an ordinary principal; the remote's acl.yaml decides; no new credential or route (DEC-QKLMMD4).
- Mass delete guard in both directions; a hidden remote row looks deleted, so the guard and the empty-type refusal are the defence; documented.
- DELETE If-Match: the ETag check runs after the read gate, so a hidden row is still a 404, never a 412.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** AC1, 2, 3 (hold), 5: dataentry two-instance e2e (remote =
data-entry server behind header proxy, as remote_push_e2e_test.go). AC3, 4, 7,
8: remotesync unit tests with pushSource fake (extended with DELETE). AC6:
dataentry handler tests. AC5: unit for per-run, 24 h window, empty type,
allow-gone, rest-of-run applies.

**Edge Cases:**
- Both sides deleted the row: ledger forgets it, no request.
- Remote answers 404 to the DELETE: treated as done.
- Local delete of an entity with linked relations: relations sent first; remote cascade removes the rest.
- Dry run: counts only, writes nothing, ledger unchanged.
- Row without a known ETag: fetched first.
- send_deletes turned off with rows in deleting: they become dropped (nothing sent).
- Deleting row whose endpoint type is no longer mapped: left alone like today.

**Negative Tests:**
- send_deletes without push: config error.
- 403 from remote: held, not retried until resolved.
- Stale If-Match: 412, row intact.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Mass delete from a remote ACL change: shared guard, empty-type refusal, off by default.
- Guard behaviour change (rest of run proceeds) on one-way remotes: tests updated; scheduler still sees ErrGoneGuard as a failed run.
- run.go is large (plimsoll/archplan): put the send phase in a new `deleter` type in deletes.go.
- Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs-project GUIDE-remote-sync-two-way and GUIDE-remote-sync (send_deletes, on_gone on two-way, guards, delete conflicts)
- [x] API reference / OpenAPI: If-Match and 412 on DELETE
- [x] CLI help for --allow-gone

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-VE812QV (significant, addressed: fetch and
compare mapped fields with the kept base, If-Match on the fetched ETag),
RR-8HKXKSG (minor, deferred: web hides entries of locally deleted rows),
RR-FA92E95 (minor, deferred: undo after a sent delete).
