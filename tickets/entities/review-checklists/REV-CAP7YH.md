---
id: REV-CAP7YH
type: review-checklist
title: 'Review: Lua capability gating (http, ai, secrets, write_file)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — `go test ./...` 0 failures
- [x] Lint clean (`just lint`) — 0 issues project-wide. NOTE: the recipe is
      OOM-killed at default memory settings in this environment (also on an
      unmodified tree). Run it as
      `GOMEMLIMIT=12GiB GOGC=400 golangci-lint run --concurrency=2 ./...`
- [x] Coverage maintained (`just coverage-check`) — package + total PASS, 77.8%

Also run: `just arch-lint` OK, `just plimsoll` clean.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-CAPSCH (critical), RR-CAPAUT (critical),
RR-CAPMCP (significant), RR-CAPGRD (minor) — all `addressed`.

The review found **two real capability-grant drops**, both of which made a
declared `capabilities:` block a silent no-op:

1. **RR-CAPSCH** — an unconditional `WithCapabilities` option erased deps-carried
   grants, so every SCHEDULED TASK's block did nothing.
2. **RR-CAPAUT** — the metamodel→internal Action conversion dropped the field, so
   every AUTOMATION's block did nothing.

Both failed CLOSED (a broken feature, never an exposure), and both had the same
root cause: the grant was hand-copied field-by-field at five sites. The fix
consolidates every translation behind `metamodel.Capabilities.Fields()`, so
adding a capability is a compile error at each consumer instead of a silent
per-surface omission.

The reviewer also cleared, by inspection: slice aliasing across the three
Capabilities types (read-only at every use), concurrency (per-request runtimes,
value-copied caps), the `"*"` non-wildcard behaviour, the trusted-surface call
list, and the reader/writer `write_file` split.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. Ungranted runtime reaches nothing — PASS
   (`TestCapabilities_ZeroValueDeniesEverything`, reader + writer)
2. A named secret does not expose the others — PASS
   (`TestCapabilities_SecretsAreNamedNotAllOrNothing`: `slack` granted,
   `db_dsn` withheld from the same file)
3. Grants are independent — PASS (`TestCapabilities_GrantsAreIndependent`)
4. A config-declared grant reaches the runtime — PASS, and this is the criterion
   that initially FAILED on two surfaces (RR-CAPSCH, RR-CAPAUT). Now pinned per
   surface: `TestLuaScriptRunner_CapabilitiesFlowFromAction`,
   `TestCapabilitiesSurviveActionConversion`,
   `TestDepsCapabilitiesSurviveExecuteFile`.
5. Operator-shell paths keep working — PASS (`scripts/generate-docs.sh`, a CI
   job, produces byte-identical output)

Every gate and every guard was mutation-tested — each was broken to confirm the
intended test fails, including the guard test itself.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CAP7YH

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/1385
