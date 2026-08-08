---
id: REV-UF5PRS
type: review-checklist
title: 'Review: Lua scripts cannot distinguish an ACL-redacted property from a genuinely-unset one'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`go test ./...` — clean)
- [x] Lint clean (`just lint` — 0 issues)
- [x] Coverage maintained (`just coverage-check` — PASS, 76.9%)
- [x] `just arch-lint` — OK, no warnings
- [x] `just plimsoll` — OK (Entity at 7 fields vs the 20 cap)

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-VXZEUN (significant), RR-TD74AU (significant),
RR-Q1VCKR (minor), RR-4DG4KF (minor) — all `addressed`.

Design-review responses from the earlier phase: RR-79L852, RR-1G0T3F, RR-KBWJPV
(addressed); RR-Q2ZRSP, RR-0A3JYK (withdrawn as incorrect after checking them
against the code); RR-IHWEB0 (open — documents a pre-existing scheduler
limitation, out of scope).

**The reviewer found two things I had missed:**

1. **RR-TD74AU** — `Redacted` reaches command-script stdin.
`commandInput` embeds the domain `entity.Entity` and marshals it whole, so this
is the one production surface where the field leaves the process. I had checked
the v1 serializers and MCP DTO and concluded "explicit DTOs everywhere", missing
that this surface has no DTO. Kept deliberately (it is useful, names-only, and
`Inaccessible` already ships there identically), now documented and pinned by a
test.
2. **RR-VXZEUN** — my own `TestDomainRedactedNotOnWire` was **vacuous**.
It asserted against a hand-built helper that structurally could not carry the
field, so it could never fail. Rewritten to drive the real
`entitySerializer.toV1`, then **mutation-tested**: injecting a leak makes it
fail, reverting makes it pass.

**Self-review also caught three invented review-response IDs** cited in new code
comments (`RR-JHQ2CX`, `RR-6NPFHG`, `RR-2QMTQF` — none existed). Cross-checked
every `RR-` reference in the diff against `tickets/entities/review-responses/`;
all now resolve to real entities.

## Acceptance Verification

- [x] Each acceptance criterion tested (see planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| 1 — redacted distinguishable from unset | PASS | `TestScriptReads_RedactedIsDistinguishableFromUnset` — both read `nil`, only the hidden one reports redacted |
| 2 — value still unreachable | PASS | `TestScriptReads_RedactedNeverCarriesValues` — value absent via `redacted`, `properties`, and `prop()` |
| 3 — nothing-hidden unchanged | PASS | `TestRedact_NothingHiddenIsUnchanged` — original pointer, no marker |
| 4 — validator does not skip | PASS | `TestRedactedDoesNotLock`, `TestRedact_DoesNotLock` — `IsLocked()` false, so `validator.go:198` cannot skip |
| 5 — no bogus git-crypt 422 | PASS | same `IsLocked()` invariant (`write_handler.go:335` keys on it) |

End-to-end through real production wiring: `TestACLScript_RedactedVisibleToLua`
drives `App.scriptReader(appRedactor(app))` with a real `acl.Declarative` +
affordance resolver — the same path document rendering uses.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-V8K1NC

Edits made to `docs-project/entities/guides/` (the generated-docs SOURCE) and
regenerated with `just docs` — `docs/*.md` is auto-generated and an earlier
revision of this work edited it directly, which would have been silently lost.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

The pre-existing `TODO(body-redaction)` in `policyreader.go` is deliberately
untouched — out of scope, and the comment already names this spot as the place a
future body-redaction change must land.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** not yet created. Work is committed on a branch; opening the PR and
watching CI is the remaining step before `done`.
