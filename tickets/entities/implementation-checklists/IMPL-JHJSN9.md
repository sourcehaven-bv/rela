---
id: IMPL-JHJSN9
type: implementation-checklist
title: 'Implementation: Skip scheduled mail when no section has content visible to the recipient'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

**Changes** (5 files, +553/-49):

| File | Change |
| --- | --- |
| `internal/mailtemplate/mailtemplate.go` | `Template.RequireVisibleContent`; `Build` returns a contribution count; section loop extracted to `buildSection` + `appendEntity` |
| `internal/appbuild/scheduled_mail.go` | Consume the count, suppress before rendering, `skipEmptyContent` log helper |
| `internal/mailtemplate/mailtemplate_test.go` | Contribution-vs-match table, YAML boolean forms, strict-decoder guard |
| `internal/appbuild/scheduled_mail_acl_test.go` | Visibility matrix, partial visibility, log non-disclosure |
| `docs/mail.md` | "Skipping recipients with nothing to read" |

Suppression is placed **before** `mailrender.New`/`Render`, so a discarded
message's untrusted content never reaches the sanitizer.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The integration matrix reuses the existing `mustScheduledMailACL` fixture so
cases differ **only** in the recipient's policy and the opt-in flag.

One deviation, deliberate:
`TestRunScheduledTemplate_RequireVisibleContentSendsOnPartialVisibility` defines
a **local** policy rather than reusing the shared one. First attempt widened the
shared `lead` role to grant `note`, which broke the two pre-existing redaction
tests — the shared role is load-bearing for what they assert. A local policy
keeps this case's needs from leaking into theirs.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Drove the real `Parse` → `Build` path against the ticket's own Atlas scenario
(`mt_agenda`, `style: detail`, `require_visible_content: true`) via a temporary
in-tree test, since `internal/` cannot be imported from a scratch module.
Output:

```text
parsed require_visible_content = true

recipient sees nothing (ACL hid every row)  contributed=0 -> SUPPRESS  subject="MT-agenda 2026-09-02 (0 punten)"
recipient sees matching-but-empty agenda    contributed=0 -> SUPPRESS  subject="MT-agenda 2026-09-02 (1 punten)"
recipient sees a real agenda                contributed=1 -> SEND      subject="MT-agenda 2026-09-02 (1 punten)"
```

Row 2 is the RR-K7RMIC case and the evidence that the two counters stayed
distinct: `{{count}}` renders **1** (one entity matched) while `contributed=0`
suppresses the send. Under the original plan this row would have sent a mail
whose only content was "Nothing to show." The temporary file was deleted after
capturing this.

**Per-criterion:**

| AC | Verified by | Result |
| --- | --- | --- |
| 1 — ACL filtering unchanged | Pre-existing `_RedactsPerRecipientACL`, `_RedactsFieldOnAVisibleRow` run unmodified | PASS |
| 2 — no send when all empty | `fully_hidden_is_suppressed_when_opted_in` (alice, denied `task`) | PASS |
| 3 — default unchanged | `fully_hidden_still_sends_when_opted_out` asserts the mail still sends AND still contains "Nothing to show." | PASS |
| 4 — diagnostic, no disclosure | `SuppressionLogDoesNotDiscloseHiddenContent`: log names template+recipient; excludes title, secret value, and entity ID | PASS |
| 5 — full matrix | hidden / partial / visible / opted-out, plus the `detail` unit table | PASS |

**Mutation check** (does the test actually catch the bug?): reverted
`appendEntity` to count matches unconditionally —
`TestBuildCountsContributionsNotMatches` failed on both the blank-body and
whitespace-body cases (`expected: 0, actual: 1`). Restored; green.

**Edge cases:** blank and whitespace-only `detail` bodies (unit table);
`list`/`table`/default contributing regardless of body; one empty section
alongside a filled one (both unit and integration); zero matches anywhere.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`skipEmptyContent` mirrors the adjacent `skipBadAddress` in shape while
deliberately differing in level — `Info`, not `Warn`, because suppression is
configured intent rather than a defect (RR-0W5FHK).

`Build`'s decomposition into `buildSection` + `appendEntity` was **not**
pre-planned: adding the counter pushed cognitive complexity to 34 (limit 30).
Extracting along the existing per-section seam was preferable to a `nolint`.

Not silent: suppression returns `nil` because a suppressed send is a completed
job, and it logs so the outcome is recoverable. Genuine errors still propagate.

**Gates:** `go build ./...` OK · `go test ./...` all pass · `go test -race` on
both changed packages OK · `just lint` clean for changed files · `just
arch-lint` OK · `just comment-lint` OK (11,653 comments, no unresolvable links)
· `just coverage-check` PASS (78.6%).

One **pre-existing, unrelated** lint failure remains on
`internal/tenant/config.go:118` (`SA4023`). Confirmed present on a clean `HEAD`
worktree, untouched by this branch; not fixed here as it is outside this
ticket's scope.
