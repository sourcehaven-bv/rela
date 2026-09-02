---
id: REV-VCBY8N
type: review-checklist
title: 'Review: Replace producer-side entitymanager.EntityManager with per-consumer interfaces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` **exit 0**, zero failures,
re-run after every review fix. Also compiled under all four backend tags
(default, `postgres`, `memorybackend`, `sqlite`), since `appbuild`'s recipes
differ per tag.
- [x] Lint clean (`just lint`) — **0 issues**. It caught two things during the
work: a `gocritic dupImport` (the same package imported twice under two names,
RR-2RCGDV) and an `lll` overrun in a new signature.
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links
across 11,460 comments. This gate **earned its keep**: deleting the type broke a
`[EntityManager]` doclink in `manager.go`, which is exactly the "godoc renders
the brackets literally, and no other linter catches it" case the rule exists
for. Fixed the comment rather than suppressing.
- [x] Coverage maintained (`just coverage-check`) — see Acceptance Status.
- [x] `just plimsoll` — passes. Also earned its keep: the first placement of
`appEntityWriter` landed between `//plimsoll:max-methods=86` and `type App
struct`, detaching the directive and failing the gate with *"type App has 86
methods, over the load line of 40"* (RR-GE0DM1).
- [x] `just arch-lint` — `OK - No warnings found`.

**Comment findings.** `just comment-report` flagged a `duplication` finding my
diff **introduced**: the same "declared here at the consumer per CLAUDE.md; the
wiring site supplies `*Manager`" boilerplate restated across three of the new
interfaces (56% shared). Per the rule's own remedy, the fact was hoisted to the
type they all cite — the `internal/entitymanager` package doc — and each
interface now points at it instead of restating it. Residual overlap is 33%,
which is just the shared pointer sentence; compressing further would remove the
pointer itself. No suppressions were added anywhere in this diff.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 12 findings — 1 critical, 6 significant, 3 minor, 2 nit.
**All 12 addressed** (none deferred or wont-fix).

RR-ZTWK9S (critical), RR-XHC5EB, RR-LOZBZQ, RR-EE4DIU, RR-09AUCP, RR-1XW25U,
RR-NEE4FC (significant), RR-GE0DM1, RR-2RCGDV, RR-PRY567 (minor), RR-VXK77U,
RR-ZOJCR3 (nit).

**The critical finding is worth reading (RR-ZTWK9S).** Review found a latent
nil-pointer panic that this refactor made reachable. `NewEngine`'s godoc said
*"applier may be nil for a push-only run (push never writes locally)"* and
`buildSyncEngine` echoed it — **both false**. `push.go`'s `recordCreate`
dereferences the applier on the id-adoption path (`newID != ch.Key`), which is
the *ordinary* create path when the primary mints its own id. `Pull` and `Force`
guarded it; `Push` did not.

It was unreachable while the CLI held a concrete `*entitymanager.Manager`.
Narrowing that field to an interface made the assertion genuinely fallible — and
the assertion **failed open**, setting `applier = nil`, which CLAUDE.md names
explicitly: *"never substitute a no-op or sentinel implementation silently."*

Fixed in three layers rather than patched at one:

1. **Removed the fallible assertion.** `writeServices` now carries a typed
`SyncApplier syncclient.LocalApplier` field assigned from the concrete manager
at the wiring site, so a missing applier is a compile error rather than a
silently disabled `pull` (RR-XHC5EB).
2. **Guarded the deref** in `recordCreate`, matching how pull and force already
behave, and renamed `errRemoteApplierRequired` → `errLocalApplierRequired` since
push qualifies too.
3. **Corrected the contract doc** — the nil case is "a run that never writes
locally", which is narrower than "push-only".

**Verified the fix is real**: `TestRecordCreate_NilApplier_ErrorsNotPanics`
passes with the guard, and reverting the guard makes it *panic with a nil
pointer dereference*. The original regression test was replaced because it
asserted only that `*Manager`'s method set was intact — a fact that could not
realistically break (RR-LOZBZQ).

**Two review findings changed the design, not just the code:**

- **RR-NEE4FC** — `appEntityWriter` was 8 methods, but App only calls 7;
`ValidateCreate` was there solely to feed `writeHandler`. That reintroduced
"wide because it is a distributor" one level down. Fixed by having `NewApp` take
the **concrete** `*entitymanager.Manager` and pass that to the sub-handler
constructors, each narrowing at its own field. `appEntityWriter` is now exactly
7 — verified equal to App's direct-call set with no slack.
- **RR-EE4DIU** — inserting `entityMutator` between `writeHandler`'s doc comment
and its declaration meant **`writeHandler` silently lost its doc comment**
entirely (Go attaches a comment to the following declaration). Neither blocking
gate catches comment *attachment*. Moved the interface above the block.

Three findings were confidently-wrong comments I had written (RR-1XW25U:
`appEntityWriter` claimed a `lua.Mutator` "floor" that constrains nothing and
omits `UpdateRelation`; RR-09AUCP: `entityMutator` stated a PatchEntity
rationale that the CalDAV writer falsifies three files over; RR-PRY567:
CLAUDE.md still told readers to depend on the deleted type). All corrected.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — interface removed. PASS.** `grep -rn "entitymanager.EntityManager"
internal/ cmd/` returns zero matches, including comments and `_test.go`.
- **AC2 — narrow interfaces at each consumer. PASS.** Six declared:
`attachment.EntityUpdater` (1), `mcp.EntityWriter` (6), `cli.entityWriter` (8),
`dataentry.appEntityWriter` (7), `dataentry.entityMutator` (7),
`dataentry.entityProvisioner` (1). Each verified against its consumer's actual
call set.
- **AC3 — build + tests unchanged. PASS.** `go test ./...` exit 0; builds under
all four backend tags; no adapter or assertion needed — `*Manager` satisfies
every interface structurally.
- **AC4 — arch-lint. PASS.** `OK - No warnings found`.
- **AC5 — no behaviour change. PASS.** No pre-existing `_test.go` was modified.
The only test changes are additions: one new regression test, and one deleted
(the weak assertion test review rejected).
- **AC6 — coupling actually dropped. PASS.** `go list -deps` confirms neither
`internal/attachment` nor `internal/mcp` depends on `internal/entitymanager` any
more. Both did before.

**Beyond the ACs — every narrowed write path exercised for real**, against a
copy of the `tickets/` project rather than only in tests: all 8 CLI methods
(`create`, `update`, `link`, `unlink`, `rename id`, `delete`) and all 6 MCP
tools driven over real JSON-RPC stdio with a full `initialize` handshake
(`create_entity`, `update_entity`, `create_relation`, `delete_relation`,
`rename_entity`, `delete_entity`) — all returning success, no `isError`. Re-run
after the review fixes.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal
refactor, `kind=refactor`. No user-facing surface changes — no CLI flag, no API,
no config, no metamodel feature.)
- [x] ~~User-facing documentation updated~~ (N/A: same reason. The doc changes in
this diff are all godoc and CLAUDE.md — developer-facing.)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A — internal refactor.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed — none added.
- [x] Ready for another developer to use — the `internal/entitymanager` package
doc now carries a "No interface here" section explaining why the wide interface
was deleted and what replaced it, so the next person does not re-add it. It also
records the distributor rule that RR-NEE4FC surfaced.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design, not
      skipped: `/pr` gates on the ticket already being `done` and validating
      clean, and a `done` checklist may have no unchecked items — so this item
      can only be satisfied by a PR that does not exist yet. Same ordering the
      note below records for the PR URL and CI status. The PR is opened
      immediately after this checklist closes.)
