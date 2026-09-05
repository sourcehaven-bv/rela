---
id: REV-44E4MQ
type: review-checklist
title: 'Review: Audit already-deleted relations when a cascade delete fails partway'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` **exit 0**, re-run after
every review fix. Builds under all four backend tags.
- [x] Lint clean (`just lint`) — **0 issues** (caught two `testifylint`
`require-error` violations in the new tests).
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links.
- [x] Coverage maintained (`just coverage-check`) — see Acceptance Status.
- [x] `just arch-lint`, `just plimsoll`, `just lint-md` — all clean. Plimsoll
**earned its keep**: the new index-pruning helper pushed `FSStore` from 81 to 82
methods, one over its pinned line. Rather than bump the number, the helper
became a package function — it needs only two fields, so it never wanted to be a
method (CLAUDE.md: "prefer splitting the type over raising the number").

**Comment findings.** No new advisory finding introduced; no suppression added.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 7 findings — 1 critical, 3 significant, 2 minor, 1 nit.
**All 7 addressed**, none deferred.

RR-UE2XS7 (critical); RR-JA8WRT, RR-M3SEHY, RR-17Y380 (significant); RR-UU2QP1,
RR-TYG8OV (minor); RR-E6VJI5 (nit).

**The critical finding is that I reproduced #929's own bug one layer down
(RR-UE2XS7).** My first version returned a partial result from the
attachment-dir abort with `DeletedEntities` empty — and added a comment
asserting that could never be wrong. It was wrong: by the time
`removeAttachmentDir` can fail, the **entity file has already been removed**
successfully. So the entity was gone and the result denied it. I had
mutation-tested the relation path and not that one, which is exactly why the
false comment survived.

Fixed by making attachment-dir removal **non-fatal**. At that point the entity
file and every relation file are gone, so the delete has materially succeeded;
returning an error would report failure for an operation that worked *and* leave
the log denying a real deletion. An orphaned attachment directory is the lesser
evil, and `analyze` already reports those. The abort-path comment now states an
invariant that is exactly rather than approximately true.

**Two more real problems review caught:**

- **RR-JA8WRT** — making `DeletedRelations` non-empty on the error path and
consuming it for *audit only* broke RR-181AFY, which had established it as the
single source for **all** relation-delete capture. The audit log would have said
the relations died while version history had no delete marker — and per
RR-181AFY the rows are off disk, so no sweep could backfill them. Half-fixing
observability is worse than not fixing it: one log tells the truth, the other
doesn't, and they disagree. `auditPartialCascade` became `recordPartialCascade`
and now emits both.
- **RR-17Y380** — an audit row asserting relation R1 was deleted, while
`ListRelations` would still return R1 until process restart. The divergence
pre-existed, but publishing a durable claim that contradicts live queryable
state made it materially worse. The store now prunes the index entries for what
it actually removed before returning the partial result.

**RR-M3SEHY** is the one I'd have missed longest: the test double fabricated a
relation that did not exist in the backing store, so the manager's incident set
was empty and `authorizeCascadeRelations` **never ran** — the test would have
passed with the ACL gate deleted. Now seeded properly.

**Found during my own caller sweep, not by the reviewer:** `cascadeHost` had the
identical audit gap on the `if_exists: replace` path, so a direct delete and a
replace would have logged differently for the same store failure. Fixed and
pinned by `TestAudit_PartialCascadeDelete_ReplacePathAlsoAudits`.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — store reports the partial set. PASS.** Mutation-verified: reverting to
`return nil, err` fails both fsstore tests.
- **AC2 — manager audits it. PASS.** Mutation-verified: removing the call fails
the integration test. That test also caught a real gap immediately —
`deleteEntityInTx` was discarding the result before the outer `res` was
assigned, so a store-level unit test would have passed while the feature did
nothing end to end.
- **AC3 — no over-reporting. PASS**, after the RR-UE2XS7 fix. Zero
`delete-entity` records on the two genuine abort paths; the attachment path no
longer aborts at all.
- **AC4 — success path unchanged. PASS.** No existing test edited; full suite
green.
- **AC5 — contract, not requirement. PASS.** `storetest` unchanged;
pgstore/sqlitestore are not newly obliged and their suites still pass.

**Every new test is mutation-verified** — five of them, each confirmed to fail
when the code it pins is reverted. For a change whose entire subject is "the log
must record what happened", a green assertion proves nothing unless it is known
to go red when the log stays silent.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated —
`docs-project/entities/guides/GUIDE-audit-log.md` (source) → `docs/audit-log.md`
(generated); regeneration touched only that file.
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-I51ZDJ

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — the store contract now states the
partial-result rule in the direction callers need (populated **iff** the entity
file was removed), and the comment at the abort paths explains why each returns
what it does.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design:
`/pr` gates on the ticket already being `done` and validating clean, so this
item can only be satisfied by a PR that does not exist yet.)
