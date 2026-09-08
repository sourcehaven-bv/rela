---
id: REV-6QPA6M
type: review-checklist
title: 'Review: Skip scheduled mail when no section has content visible to the recipient'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go build ./...` — clean
- [x] `go test ./...` — full suite, exit 0
- [x] `go test -race` on both changed packages — clean
- [x] `just lint` — 0 issues on `internal/mailtemplate` + `internal/appbuild`
- [x] `just arch-lint` — OK, no boundary violations
- [x] `just comment-lint` — OK (11,653 comments, no unresolvable doc links)
- [x] `just coverage-check` — PASS, total 78.6%

One **pre-existing, unrelated** lint failure remains:
`internal/tenant/config.go:118` (`SA4023`). Verified present on a clean `HEAD`
worktree and untouched by this branch, so not fixed here.

## Code Review

Two reviewers were run. The security review completed; the general code review
**stalled twice** (600s watchdog, no findings produced), once while contending
with a concurrent full test run and again on a second attempt. Rather than a
third attempt, its last emitted lead — the sections-less template case — was
investigated directly and produced a real finding (RR-RV093C).

**Security review (`rela-security-reviewer`) — no findings.** Verified:

- Read path unchanged: `deps.VisibleReader` still threaded, `Build` its sole
caller, the refactor moved the loop body verbatim, no new reader or `s.store`
access. The raw `GetEntity` result still flows only into the envelope.
- Log line clean: template name (non-secret config per CLAUDE.md) + recipient ID
only; neither `model` nor `contributed` is in scope at the call site.
- Membership channel is *weaker* than before, not stronger — with the flag off,
`{{count}}` already interpolated the match count into a delivered message.
Suppression discloses no values.
- Render order preserved: suppression sits strictly before `mailrender.New`;
`internal/mailrender` and `internal/mail` are byte-for-byte untouched.
- `KnownFields(true)` intact; the new bool has no custom unmarshaler, so a
non-boolean scalar fails the whole load (fail-closed).
- No timing or cross-principal oracle: `Build` runs once per recipient with that
principal on ctx; nothing is cached or shared between invocations.

The reviewer also flagged a transient on-disk state it observed mid-review (a
spurious `list` guard). That was investigated and is **not** in the tree — see
the note below.

**Findings:** 1 (minor), addressed.

| ID | Severity | Status |
| --- | --- | --- |
| RR-RV093C | minor | addressed |

RR-RV093C: `sections:` is optional, so `require_visible_content: true` on a
sections-less template produced a config that parsed, validated, scheduled, and
then silently discarded every send forever. Verified against the real `Parse` →
`Build` path (`sections=0 contributed=0`, intro non-empty, suppressed). Fixed as
a load-time error, with a control assertion proving a sections-less template
*without* the flag stays valid.

## Verification

- [x] Each acceptance criterion re-verified after the RR-RV093C fix

| AC | Evidence | Result |
| --- | --- | --- |
| 1 — ACL unchanged | Pre-existing `_RedactsPerRecipientACL` / `_RedactsFieldOnAVisibleRow` pass unmodified; security review confirmed the read path by inspection | PASS |
| 2 — no send when all empty | `fully_hidden_is_suppressed_when_opted_in` | PASS |
| 3 — default unchanged | `fully_hidden_still_sends_when_opted_out` asserts the mail sends AND still contains "Nothing to show." | PASS |
| 4 — diagnostic, no disclosure | `SuppressionLogDoesNotDiscloseHiddenContent` pins positives and negatives | PASS |
| 5 — full matrix | hidden / partial / visible / opted-out + the `detail` unit table | PASS |

Additionally verified against the real CLI: `rela validate` reports `cannot
unmarshal !!str 'true' into bool` for a quoted value and accepts the bare form,
so the new key is covered by existing validation for free.

**Note on transient file states.** During the review window both a reviewer and
I observed `internal/mailtemplate/mailtemplate.go` and
`internal/appbuild/scheduled_mail.go` briefly holding content neither of us
wrote (a `list`-branch guard; a suppression condition missing its
`tmpl.RequireVisibleContent &&` clause). Each was accompanied by concurrent
long-running Go builds. The committed state was confirmed correct by `git diff`
and by repeated clean `-count=1` runs, and the tests caught both variants when
they were momentarily present — which is the outcome the AC 3 control test
exists for.
