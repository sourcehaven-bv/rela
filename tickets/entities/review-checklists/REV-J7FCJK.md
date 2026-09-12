---
id: REV-J7FCJK
type: review-checklist
title: 'Review: Replace EasyMDE with Milkdown (ProseMirror) in data-entry forms'
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

| Gate | Result |
| --- | --- |
| Go tests | 104 packages, 0 failures |
| Frontend unit | 2,597 tests, 160 files |
| E2E (Playwright) | 290 passed, 8 skipped, 0 failed |
| Corpus sweep (`RELA_FULL_CORPUS=1`) | 3,939 files, 0 semantic drift, 0 non-idempotent |
| `just comment-lint` | no unresolvable doc links across 13,852 comments |
| `just arch-lint` | OK, no warnings |
| `just plimsoll` | clean |
| `just coverage-check` | PASS, total 79.6% |
| `go vet` | clean |

ESLint reports 0 errors. The 4 warnings on `DynamicForm.vue` (file length, three
non-null assertions) are pre-existing in a 1,591-line file this change adds two
lines to. New files are warning-free.

**Comment findings.** No new advisory findings introduced; no suppressions
added.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-6OIIOE, RR-P3K8UG, RR-HDEVDK (critical); RR-R82OEB,
RR-1KPRKU, RR-X4K509, RR-XN8DTF, RR-7NNHBN, RR-DC30OF, RR-JUDQ5Q, RR-N8CNLQ,
RR-IS01DT (significant). All `addressed`.

Both a design review and a code review ran. Three criticals, all data-integrity
defects that the passing test suite did not see:

- the write-back guard was never on the save path (RR-6OIIOE)
- its baseline was overwritten at load, so it could not have worked anyway
(RR-P3K8UG)
- a reference inserted and immediately submitted was lost to the listener
debounce (RR-HDEVDK)

Each fix is verified by reintroducing the bug and confirming the right tests
fail. RR-IS01DT is a pre-existing bug on `develop`, fixed here because it
otherwise leaves the e2e suite red and reads as a Milkdown regression.

**Unrelated changes:** one, deliberate. `@milkdown/vue` was added as a
dependency and never imported (the component builds on `@milkdown/kit`); it is
removed rather than shipped.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. **Opening and saving an untouched entity emits nothing** — PASS. Full corpus
sweep clean; four component tests over churning body shapes assert no emit and
original bytes returned; e2e opens a body with a setext heading, mixed bullets,
a table and a reference and asserts the stored bytes are unchanged. This
criterion genuinely failed until RR-6OIIOE and RR-P3K8UG were fixed.
2. **A reference renders as a titled link and serializes to its code span** —
PASS. E2E asserts both the rendered link and that the stored markdown does not
contain the title.
3. **No title leaks through the read gate** — PASS. Go tests, proven
non-vacuous by substituting `visibility.AllowAllReader`: both LEAK assertions
fire with the hidden title in the body. RR-R82OEB additionally closed a case
where a title outlived a revoked grant.
4. **App-editor bundle unchanged** — PASS. Still builds on EasyMDE; the SPA
bundle contains no `EasyMDE`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-NB44J8

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

One thing a reader should know rather than rediscover: **Milkdown's markdown
listener is debounced by 200ms and does not fire at all under happy-dom.** That
is why unit tests could not see the two save-path defects, and why any future
change to the emit path needs either an e2e test or a test that drives the
registered callback directly.

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR post-dates
      this checklist — `/pr` gates on the ticket already being `done` and
      validating clean, so this item cannot be satisfied before it runs. See
      the note below and TKT-UFV01M.)
