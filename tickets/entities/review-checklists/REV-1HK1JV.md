---
id: REV-1HK1JV
type: review-checklist
title: 'Review: Faced relation history: capture skipped state-tailed edges and reads addressed the default tail'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — default suite with `-race`, plus the
sqlite-tagged and postgres-tagged suites against a local database. CI: Test,
SQLite Backend, Postgres Backend, E2E, Frontend, Fuzz all pass.
- [x] Lint clean (`just lint`) — 0 issues. CI Lint passes.
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links
across 15113 comments.
- [x] Coverage maintained (`just coverage-check`) — 79.6%, both thresholds pass.

**Comment findings.** No advisory findings introduced by this diff. Three
golangci findings exist in `internal/store/pgstore/cas_crossprocess_test.go`
under the postgres build tag (misspell, modernize, testifylint); they are
pre-existing, in a file this change does not touch, and CI runs `golangci-lint
run ./...` without build tags so they are not gated.

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: the cranky-code-reviewer pass ran
against the BUG-64MU2Q work in this same branch and produced RR-5MLZCR and
RR-OP54MI, both addressed. This ticket's changes were reviewed against the same
diff.)
- [x] All critical review-responses addressed — RR-5MLZCR (incoming faced
delete addressed the zero tail) fixed in `8f48fe32`.
- [x] All significant review-responses addressed — RR-OP54MI (copy engine and
cascade delete dropped the face) fixed in `00fb2088`.
- [x] Self-reviewed the diff for unrelated changes — the diff is confined to
the relation face/tail path. One incidental fix is included deliberately and
documented on the ticket: `sqlitestore.WriteRelationVersion` never resolved a
record id, so every synchronous capture landed on lineage 0.

**Review Responses:** RR-5MLZCR, RR-OP54MI (both from the BUG-64MU2Q review of
this branch; no new findings for this ticket).

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- *A faced relation's version capture lands on its own lineage* — PASS.
`storetest` `RelationTails/TailsHaveIndependentLineages`, run on both database
backends. Verified to FAIL before the fix: both captures landed on
`rel_record_id = 0`, one lineage holding two versions.
- *Two tails holding identical bytes stay distinct* — PASS.
`RelationTails/IdenticalContentAcrossTailsStaysDistinct` asserts different
content hashes.
- *A tail with no live edge and no history is refused, not misfiled* — PASS.
`RelationTails/UnknownTailIsNotFound` asserts `store.ErrNotFound`.
- *History reads resolve by face* — PASS. The same suite reads each tail by
face and asserts it gets that tail's snapshot.
- *A faced address on the HTTP route reaches its own tail; a bare one reaches
the default tail* — PASS, both directions:
`TestRelationHistory_FacedAddressReadsItsOwnTail` and
`TestRelationHistory_BareAddressReadsTheDefaultTail`.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing
surface changed. The store interface and handler changes are internal; the CLI
gained `ID@face` on an existing positional, which the flag help already
describes.)
- [x] ~~User-facing documentation updated~~ (N/A: same reason.)
- [x] ~~Docs-checklist marked as done~~ (N/A: same reason.)

**Docs Checklist:** none (no user-facing change).

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
