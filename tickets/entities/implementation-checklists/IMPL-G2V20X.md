---
id: IMPL-G2V20X
type: implementation-checklist
title: 'Implementation: Type picker and fuzzy ranking for the editor''s @ mention completion menu'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Implemented per PLAN-9TBAT2 revision 2, frontend only. No backend change, as
designed.

**New files:**

- `frontend/src/components/forms/milkdown/mentionRanking.ts` — the uFuzzy scorer
for entities and for type names, on uFuzzy's **defaults** (see RR-8I7XUD for why
`intraIns: 1` was removed). Guards: `tokenizerSawWholeNeedle` passes the server's
order through for any query uFuzzy only partly tokenizes, and `uf.filter()` is
null-checked (it returns `null`, not `[]`).
- `frontend/src/components/forms/milkdown/mentionKeymap.ts` — the menu's key
semantics, extracted from the editor (see Quality below).
- Tests for both, plus `shouldClearScopeOnBackspace` cases appended to
`mentionQuery.test.ts`.

**Modified:** `useMentionMenu.ts` (type state, `selectType`/`clearType`,
`setAvailableTypes`, identity-based highlight, `useSchemaMentionMenu` wrapper),
`MentionMenu.vue` (types section, scope chip, section headers),
`MilkdownEditor.vue` (choice dispatch, keymap wiring), `mentionQuery.ts`
(`shouldClearScopeOnBackspace`), `package.json` (`@leeoniya/ufuzzy` 1.0.19),
`docs/data-entry.md`, `frontend/CLAUDE.md`, and the e2e spec + page object.

**The `terms` floor is not present.** Per RR-NNFWGP it was dropped rather than
fixed: only uFuzzy's own `filter` removes a candidate.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Two fixture defects were found and fixed during implementation, both of which
had made tests pass for the wrong reason:

1. The `ent()` factory set a top-level `title`, but `entityDisplayTitle` reads
`_title`. Every haystack therefore collapsed to the bare ID, so a title-matching
test could not have failed. This is the same class of error as BUG-1P88YM and is
now called out in a comment on both factories.
2. The combined-highlight fixtures used entity ids unrelated to the query, so
the entity section was empty and the traversal assertions passed vacuously.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Verified against a real `rela-server` with a live bleve index (`just
build-server-e2e`), not only in jsdom.

| AC | Result |
|---|---|
| 1 picker scopes | PASS — unit asserts `searchEntities(query, 'ticket', …)`; e2e picks `feature` and inserts |
| 2 progressive disclosure | PASS — table-driven over lengths 0,1,2,5,6,7 |
| 3 backspace clears chip | PASS — e2e, traced keystroke by keystroke (see below) |
| 4 Enter unambiguous | PASS — 12 keymap unit tests + e2e |
| 5 cross-field reach | PASS — e2e `@featur` → Enter → token → insert |
| 6 fuzzy ranking | PASS — quality ordering, exact/prefix ID first |
| 7 prefix never blanks | PASS — 13-step prefix matrix over `fancy-ranking` |
| 8 non-Latin | PASS — CJK and Cyrillic pass the server order through |
| 9 `@---` | PASS — "No matches", no throw, no "Search failed" |
| 10 never invents rows | PASS — output ⊆ input asserted over 7 query shapes |

Suites (final, post-review): **2840** frontend unit tests in 173 files, all
passing (the markdown corpus round-trip test included). **303 e2e passed, 10
skipped, 0 failed** across the whole suite. The mention spec also held over
`--repeat-each=4` (84 runs) before the review fixes.

`just comment-lint` clean (Go-only gate; this diff has no Go changes).
`just coverage-check` did not complete — it was killed by signal 15 on two
attempts, both times partway through the race-enabled Go run. **This diff
contains zero Go files** (`git diff --name-only | grep '\.go$'` is empty), so Go
coverage cannot be affected by it; the frontend has no coverage enforcement by
project policy. Recorded as not-run rather than claimed as passing.

**One real bug was found by the new e2e test and fixed.** Backspace read
`menu.state.query`, which the slash provider writes on ProseMirror's *update*
cycle — one tick after the capture-phase key handler. On the keystroke that
empties the query that mirror is one character stale, so the guard fired late:
the `@` trigger was deleted, the menu closed, and `close()` reset the scope. The
symptom was indistinguishable from the chip clearing correctly. The fix re-reads
the live document via `parseMentionQuery(slashProvider.getContent(view))`.

Traced in the browser to confirm, pressing Backspace with no waits between
keystrokes:

```text
before: bs 6: text="see @"  chip=1 types=0
        bs 7: text="see "   chip=0 types=0   <- trigger lost
after:  bs 7: text="see @"  chip=0 types=8   <- trigger kept, types back
```

The timing is why this needed a browser: any pause between keystrokes lets the
mirror catch up and the bug disappears, which is exactly why an early
`--workers=1` run with 250 ms waits passed while the real suite failed. There is
no ProseMirror update cycle in jsdom, so a unit test cannot observe it at all.
The e2e test carries a comment saying so, to stop it being demoted later.

**Mutation-verified.** Reintroducing the stale-mirror read fails 3 keymap tests,
including the one named for it. The tests catch the defect rather than merely
passing alongside it.

## Post-review fixes

`/code-review` (cranky-code-reviewer plus rela-security-reviewer, run in
parallel) found **11 findings: 4 critical, 4 significant, 2 minor, 1 nit** — all
addressed. Full detail in the linked review-responses; the four criticals were
one root cause.

**The highlight was index-based, and that was wrong four ways** (RR-J3OEGA,
RR-T6SNQN, RR-0GQVYU, RR-XWOQZH). All four lived in the 150 ms window between a
keystroke and the search response, which my tests could not observe because they
all settled before asserting. Worst case verified independently: `highlight=3`
against a 2-item list, `current()` null, and since the keymap returns without
`preventDefault` on null, **Enter put a paragraph break into the user's
document** on a visibly highlighted row. Fixed structurally rather than by
clamping: the highlight now stores the row's IDENTITY and resolves it against the
live rows on read, so a stale or out-of-range index is unrepresentable.

**`intraIns: 1` was a tab-freeze** (RR-8I7XUD). Measured independently at 55
seconds of synchronous main-thread work for a 64-character needle against a
repeated-run title, versus 0 ms on uFuzzy's defaults. Removed; it only bought
single-typo tolerance, and the motivating `fancyreport` → `FancyReport` case does
not need it. A length bound was measured and rejected (the blowup starts near 16
characters).

**Two of my own mistakes worth naming.** My first fix for the partial-tokenizer
guard (RR-G3YZ8I) regressed the all-separator case, caught immediately by the
existing tests. And two of my new tests initially failed on fixture setup —
entities that did not match the query — the same class of error the reviewer had
just flagged.

**Mutation-verified, not assumed.** Reverting the scope-change fix fails exactly
its three tests; reintroducing `intraIns: 1` hangs the suite instead of failing;
reintroducing the stale-index read initially did NOT fail the test written for
it, which is why that test was strengthened to highlight the last entity rather
than the first.

## Post-demo fix: types narrow from the first letter

Found by the operator running the local demo against the real `tickets/` project
(24 types): the type list did not filter until the second character.

Cause: `refreshTypeItems` gated the type list on `MIN_SEARCH_LEN = 2`. That
constant exists to stop a one-character ENTITY search reaching the server, which
is a different concern — filtering 24 type names is local and free. The two never
needed to share a threshold. Fix: only a bare `@` (length 0) lists everything;
from the first letter the names are ranked and capped as before.

Measured against the real 24 types: `t` → test-case/test-suite/ticket, `d` →
decision/doc-task/docs-checklist, `q` → qa-report alone, `z` → no matches. Prefix
matches rank first. The entity search still waits for the second character, so no
extra request is made.

Two unit tests had pinned the old behaviour and now assert the new intent, plus
one new test for the bare-`@` case and an e2e test for single-letter narrowing.

**The first version of that e2e test failed against working code**, because it
read `count()` once immediately after the keystroke and got the pre-render list.
It now asserts on row CONTENT via `expect`, which auto-retries, and uses a type
that cannot match the letter as the witness that narrowing happened.

**Pre-existing e2e flakiness noted, not introduced.** Under `--repeat-each=3` the
editor specs fail 1-2 of ~70 with a *different* test each run. Verified against
the stashed baseline: 2 of 69 failed there too. Not caused by this change; worth
its own ticket.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Patterns followed.** `useSchemaMentionMenu` keeps the store binding out of
`useMentionMenu`, so the composable's tests need no Pinia. `mentionKeymap.ts`
declares its `MentionKeymapHost` interface at the call site (root CLAUDE.md:
"Define interfaces at the call site"), naming only the three things it cannot do
itself.

**`max-lines` was treated as the signal it is, not as noise.** `MilkdownEditor.vue`
sat at exactly 500 counted lines, so this change tripped the 500-line
god-component warning. Rather than shave comments, the menu's keyboard handling
moved to `mentionKeymap.ts` — which also made it unit-testable without mounting
an editor. Lint is now **129 → 128** warnings against baseline, and zero warnings
in any file this ticket touched. `npm run typecheck` clean; 0 lint errors.

**A stray doc comment** above `tableCommands` (it described `ALL_COMMANDS`) was
reattached to the declaration it documents.

**Security.** The type reaching `?type=` is checked against the schema list in
`selectType`, so it is an allowlist by construction and a free-typed string can
never become one. `setAvailableTypes` drops a scope that a schema reload retired.
The query is sent unmodified, preserving the single-token invariant that keeps
`@status:open` inert. The haystack uses the server's already-redacted `_title`
and no title is derived locally.
