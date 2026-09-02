---
id: REV-G3AE0A
type: review-checklist
title: 'Review: analyze: flag relation files whose filename disagrees with their content'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` **exit 0**, re-run after
every review fix.
- [x] Lint clean (`just lint`) — **0 issues** (caught three `misspell`
British-spelling slips in the new test docs).
- [x] Comment lint gate clean (`just comment-lint`) — clean.
- [x] Coverage maintained (`just coverage-check`) — adds tests, removes none.
- [x] `just arch-lint` — clean. It **earned its keep twice**: it rejected the
first version's `internal/markdown` import, and the boundary it enforced is what
led (via review) to `internal/frontmatter`, which is the correct dependency and
one this check should have used from the start.
- [x] `just plimsoll`, `just lint-md` — clean (lint-md caught an emphasis-style
and two over-length lines in the new docs).

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 7 findings — 1 critical, 3 significant, 2 minor, 1 nit.
**All 7 addressed**, none deferred.

RR-YB7XX8 (critical); RR-DDZ02R, RR-TSBK9Z, RR-OW2QSH (significant); RR-8JRYEE,
RR-DHOSFM (minor); RR-WST9B6 (nit, on the AC1.7 test).

**The critical finding is that I broke my own check and pinned the mistake.**

The first version reported 38 issues on the repo's own `tickets/` project. I
judged 37 of them false positives, "fixed" the check to suppress them, and
encoded that with two table cases asserting they were fine.

They were **true positives**. Those files spell the relation type `type:`
instead of `relation:`, and `mdCodec` builds the relation from
`doc.getString("relation")` — so they load with an **empty type** while the
index, built from the filename, says otherwise. Verified on the real project:
three relations on `BUG-2OXEW0` rendered with a blank type.

My suppressing comment claimed *"they work, because the store keys on the
FILENAME"*. The first clause was false; the second was a non-sequitur that
produced it. Indexing is not the only thing that reads a relation file.

This matters more than the shape it was hiding. #1004's corruption fails loudly
downstream as a cardinality error; this one is silently inconsistent, and a read
→ write round trip through any path (formatter, rename, migration) writes the
empty type back and destroys the last record of it.

**Fixed three ways, and the data was repaired:** the check reports them under
`ReasonLegacyTypeKey`; the tests assert that rather than the opposite; and the
**37 files were corrected** (`type:` → `relation:`) after verifying all 37
agreed with their filenames so the rename lost nothing. One more in
`docs-project`.

*The transferable lesson:* a finding you cannot immediately explain is not
automatically a false positive. Reaching for the suppression before tracing what
the store actually does with those files is what turned a correct check into a
wrong one — and a pinned test made the wrong conclusion durable.

**Two more real corrections:**

- **RR-DDZ02R** — I hand-rolled a YAML frontmatter scanner and justified it by an
arch-lint rule that does not apply. `yaml` is a declared `commonVendor` and
`internal/frontmatter` exists precisely for this. Worse than redundant: it made
a checker whose entire claim is "I read the file the way rela reads it" read it
a *third* way, and a UTF-8 BOM made it return zero keys — a planted real
mismatch passed clean. Now uses the shared splitter.
- **RR-TSBK9Z** — the deliberate copy of `fsstore.parseRelationFilename` is
correct (arch-lint forbids the dependency; exporting a storage detail to save
nine lines would be worse), but nothing kept the two identical. Now pinned in
both directions over the same table, each test naming the other. It had already
drifted once by an unreachable extra guard.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- **AC1 — the reporter's scenario is named. PASS.** A file whose content points
at a different target reports the file and **both** triples.
- **AC2 — no false positives on real data. PASS**, after the correction above.
`tickets/` and `docs-project/` both clean; the corrupted copy reports its
injected mismatch.
- **AC3 — the split agrees with the indexer. PASS.** Pinned in both packages over
an identical table, including the `A--we--ird--B` case that a naive
`Split("--")` gets wrong.
- **AC4 — the check runs where operators look. PASS.**
`rela analyze relation-files`, with per-reason rendering.

**Verified against three real projects, not just fixtures.** That is what
surfaced the critical finding — no unit test would have, because the fixture
data was of my own choosing and matched my own wrong mental model.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-23G0PV

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use — each `RelationFilenameReason`
documents its consequence, and the docs give the fix per finding.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (deferred by design:
`/pr` gates on the ticket already being `done`.)
