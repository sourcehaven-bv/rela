---
id: REV-NGEHMLX
type: review-checklist
title: 'Review: Apply pending config migrations in memory on load and warn instead of refusing to boot'
started: "2026-11-25"
completed: "2026-11-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (affected packages run: migration, metamodel, projectsetup, queryplan, dataentry, appbuild, cli, cmd/...; postgres and sqlite tags build; CI runs the full suite)
- [x] Lint clean (`just lint`) (golangci-lint on every changed package, plus arch-lint, plimsoll and comment-body)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: floors are enforced by the CI coverage job; every new function has tests)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-QJEXKXI, RR-N0VWDPO (significant, addressed);
RR-J6OY1MZ, RR-XPOEID6, RR-IIRWWXV, RR-RE0MGSM, RR-C16FGWD, RR-AX9HNXO,
RR-OFNZHVL (minor, addressed); RR-BVN3EVC (minor, deferred); RR-4RJZH2G (nit).
Design review: RR-W7HKMAM, RR-V1H3S0V, RR-89SMFRR, RR-GNEITSI, RR-BE85S83.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: schema loads via in-memory migration with a warning (TestFSLoader_MigratesDeprecatedSyntaxInMemory; manual `rela list` on a scratch project).
2. PASS: data-entry loads at boot and on reload (TestReloadConfig_MigratesDeprecatedSyntax); derived indexes too (TestLoadStaticIndexSpecsMigratesDeprecatedSyntax).
3. PASS: files on disk unchanged (byte comparison in both loader tests; manual cmp).
4. PASS: automations: still refuses (TestFSLoader_RefusesAutomations); so does boolean allow_acl_bypass (TestMigrateBytes_Refuses).
5. PASS: `rela validate` still fails (TestStrictFSLoader_RefusesDeprecatedSyntax; manual run).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-YLR01ZO

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
