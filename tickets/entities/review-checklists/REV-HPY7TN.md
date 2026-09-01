---
id: REV-HPY7TN
type: review-checklist
title: 'Review: Attachments on entity create: stage files in the create form, upload after the entity exists'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `just test` — Go `./internal/dataentry/... ./internal/attachment/...` ok;
frontend 2027/2027 across 124 files; e2e 271 passed / 8 pre-existing skips / 0
failures
- [x] `just lint` — golangci-lint 0 issues; `npm run lint` 0 errors (114 warnings,
all pre-existing); commentlint gate clean; plimsoll clean
- [x] `just coverage-check` — package and total thresholds satisfied, 78.3%
- [x] `just arch-lint` — OK, no warnings
- [x] `just docs-check` — docs regenerated from `docs-project/` and in sync
- [x] `npm run typecheck` / `npx tsc --noEmit` (e2e) — clean
- [x] `npx eslint` on the new e2e spec — clean (the page-object rule is a
compile-level error, so the spec is proven to use page objects only)

## Code Review

- [x] `/code-review` run (cranky-code-reviewer)
- [x] All critical findings addressed
- [x] All significant findings addressed
- [x] Minor/nit findings addressed or explicitly deferred with reason

**Findings — 8 total, all addressed:**

| ID | Severity | Finding |
|----|----------|---------|
| RR-HJLLUF | critical | Double-submit via Cmd+Enter creates two entities and two sets of uploads |
| RR-4QO887 | critical | Failure path promised a retry that did not exist; kept-dirty state enabled duplicate creates |
| RR-OI6P51 | significant | Staged files bypassed wizard-hidden pruning, persisting a revealed-then-hidden branch |
| RR-C6CXU1 | significant | Duplicate Vue `:key`; unstage-by-identity removed both copies |
| RR-C1MI2N | significant | Deep `ref` proxies `File` under happy-dom — tests exercised proxies, production raw Files |
| RR-7T94EF | significant | Embedded path's keeps-form-dirty guarantee was void |
| RR-QQQ2JR | significant | Missing tests: double-submit, unmount mid-upload, wizard-hidden, duplicate key |
| RR-1556G1 | minor | `canStage` looser than `canEdit`; `max: 0`; render-time mutation; duplicated error mapping |

**What the review actually changed.** Two of these were real defects the design
review did not reach, and both came from the same blind spot: adding a second
network phase to submit widens an existing race and turns "retry" into
"duplicate". The double-submit hole pre-dated this ticket, but its window grew
from one POST to a POST plus N multipart uploads — seconds, not milliseconds.
The retry promise was worse: the code kept staged files and a dirty flag for a
recovery path that the very next `router.push` destroyed, while leaving a live
form that would happily create a second entity. I took the honest option — clear
the state, say "re-attach on the entity", and fix the docs that had promised
otherwise.

RR-C1MI2N is the one worth remembering: happy-dom's `File` reports `[object
Object]`, so a deep `ref` proxies it and identity is not preserved, while a spec
`File` is left alone. I verified both halves with a throwaway test before
accepting the finding. Every unit test here was exercising proxies while
production exercised raw `File`s — `shallowRef` makes them agree.

## Acceptance Verification

Each criterion from the ticket, with evidence:

| # | Criterion | Result |
|---|-----------|--------|
| 1 | Pick/drag a file on the create form; entity saved with file attached and property stamped | **PASS** — e2e `stages a file in the create form and attaches it after save`; manual run stamped `attachments/BUG-001/screenshot/manual-shot.png` |
| 2 | A staged file removed before save is not uploaded | **PASS** — e2e `a staged file removed before save is never uploaded`; unit `does not upload a staged file that was removed before save` |
| 3 | At `max > 1`, up to `max` stage and all upload; add control hidden at capacity | **PASS** — e2e `stages several files…` and `hides the add control once a multi-cap property is full`; manual counter showed `2 / 3` |
| 4 | Upload failure leaves the entity reachable, lands the user on it, shows the server's error | **PASS** — e2e `an oversize file fails loudly after the entity is created`, driving a **real 413** (fixture `max_attachment_bytes: 1024`) |
| 5 | No `update` permission → same 403 as the edit path; no false success | **PASS** — unit 403 case; server-side gate unchanged (`attachmentWritePreflight`) and covered by existing Go `acl_*` tests |
| 6 | Uploads address only the server-returned id | **PASS** — unit asserts `uploadAttachment('bug', 'BUG-7', …)` against the mocked create response; manual paths carry the server's `BUG-001` |
| 7 | A `required` file property is not made worse | **PASS** — no change to required handling; owned by TKT-87VSDE |

## Documentation

- [x] `docs/data-entry.md` updated (via `docs-project/`) — new "File Properties
on Forms" section, corrected after RR-4QO887 so it no longer promises a retry
the code does not implement
- [x] No metamodel / CLI / CLAUDE.md changes needed
- [x] `docs/attachment-security.md` deliberately unchanged — the security model
is untouched, which is the whole argument for this approach
