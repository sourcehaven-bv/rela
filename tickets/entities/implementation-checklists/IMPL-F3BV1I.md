---
id: IMPL-F3BV1I
type: implementation-checklist
title: 'Implementation: analyze: flag relation files whose filename disagrees with their content'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — a table over eight cases in
`internal/analysis/relation_filename_test.go`, plus a dedicated test reproducing
the reporter's exact file from issue #1004.
- [x] Integration tests written — the check was run against **three real
projects** (`tickets/`, `docs-project/`, and a copy of `tickets/` with the
reporter's corruption injected). That is what caught the false positive below;
no unit test would have.
- [x] Happy path implemented
- [x] Edge cases from planning handled — unparseable filename, relation type
containing the `--` separator, non-markdown files, files with no relation
frontmatter, and the legacy `type:` key.
- [x] Error handling in place — an unreadable or unparseable file returns no
finding rather than a spurious one; that is a different problem with its own
reporting.

## Test Quality

- [x] Using fixture builders or factories — one `newFSService` helper builds the
service over an in-memory FS; `relFile` composes the frontmatter.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter — each table case declares exactly the
one file it is about.
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

*First, the half of #1004 that is already fixed.* The rename path most likely to
have produced the reporter's files is correct: after `RenameEntity(REQ-1 →
REQ-999)`, the only file present is `SOL-1--implements--REQ-999.md` —
`fsstore.renameEntity` writes each incident relation under its new name and
removes the old one. So this ticket is a **detector for legacy corruption**, not
a fix for an ongoing defect, and the issue's first suggestion needs no work.

*Second, the reporter's scenario, reproduced exactly.* A file named
`PRS-FLOW-1HL6--wordtUitgevoerdDoor--PRS-FUNC-8Q7E.md` whose content says `to:
PRS-FUNC-0Q7E` now produces a finding naming the file and **both** triples.
Reporting only one would leave the operator to guess the mapping between the
filename they must fix and the entity every other check complains about.

*Third — the finding that justified running it for real, and where I then got
it wrong.* On the repo's own `tickets/` project the first version reported **38
issues**: my injected one plus 37 others using the legacy `type:` key instead of
`relation:`. I judged those false positives, suppressed them, and pinned that
with two table cases.

**They were true positives.** Code review (RR-YB7XX8) caught it. Indexing is not
the only thing that reads a relation file: `mdCodec` builds the relation from
`doc.getString("relation")`, which returns `""` for a legacy file — so those
relations load with an **empty type** while the index, built from the filename,
says otherwise. Verified on the real project: three relations on `BUG-2OXEW0`
rendered with a blank type.

That is a worse variant of the shape this check exists for. #1004 fails loudly
downstream as a cardinality error; this one is silently inconsistent, and any
read → write round trip (formatter, rename, migration) writes the empty type
back and destroys the last record of it.

Fixed three ways: the check reports them under their own `ReasonLegacyTypeKey`;
the tests assert that instead of the opposite; and **the 37 files were
repaired** (`type:` → `relation:`), after confirming all 37 agreed with their
filenames so the rename lost nothing. One more in `docs-project`.
`BUG-2OXEW0`'s relations now render with their real type.

The lesson is narrow and worth keeping: *a finding you cannot immediately
explain is not automatically a false positive.* Reaching for the suppression
first, rather than tracing what the store actually does with those files, is
what turned a correct check into a wrong one — and encoding the wrong conclusion
as a pinned test is how it would have stayed wrong.

After the real fix: `tickets/` and `docs-project/` both report `✓ All relation
files agree with their filenames`; the corrupted copy reports its injected
mismatch.

**Gates:** `go test ./...` exit 0; `just lint` 0 issues; comment-lint, plimsoll,
lint-md clean; `just arch-lint` clean *after* the boundary fix below.

## Quality

- [x] Code follows project patterns — modelled on `CheckRelationOrder` (same
package, same service, same soft-finding shape) and on `FindOrphanedTempFiles`
for the optional FS + Paths gate, including returning nil rather than erroring
when they are absent.
- [x] Checked for DRY opportunities — `splitRelationFilename` deliberately
**duplicates** fsstore's `parseRelationFilename` rather than importing it: it is
unexported there, and arch-lint forbids `analysis → store/fsstore` anyway. The
duplication is load-bearing and commented — this check must split names
*exactly* as the indexer does, or it reports false positives on relation types
containing `--`. That case is in the table.
- [x] No security issues introduced — read-only over files the caller can
already read; no new input surface.
- [x] No silent failures — the whole point is turning a silent mis-index into a
named finding.
- [x] No debug code left behind.

**arch-lint caught a real boundary violation — and my first fix for it was also
wrong.** The first version imported `internal/markdown`; `analysis` may not
depend on it, and the rule is right. I replaced it with a private ~20-line
frontmatter scanner, justified in its godoc by that same arch-lint rule.

Review (RR-DDZ02R) pointed out the justification did not apply: `yaml` is a
declared `commonVendor` importable by every component, and `internal/frontmatter`
exists precisely for this — a dependency-free leaf that `markdown` and `fsstore`
already share. Worse than redundant, the scanner made a checker whose entire
claim is *"I read the file the way rela reads it"* read the file a **third** way;
a UTF-8 BOM made it return zero keys, so a planted real mismatch passed clean.

Now `frontmatter.Split` + `yaml.Unmarshal`, with one line adding `frontmatter`
to `analysis.mayDependOn`.