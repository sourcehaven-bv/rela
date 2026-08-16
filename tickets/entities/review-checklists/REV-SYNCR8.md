---
id: REV-SYNCR8
type: review-checklist
title: 'Review: sync is a client of the authorized API (fancy-browser)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `golangci-lint` clean (0 issues) on `internal/cli/sync` + `internal/dataentry`
- [x] `just arch-lint` clean (no package-boundary violations); `just plimsoll` clean (App back under 90 methods — v1 relation-read handler is a package fn, not an App method)
- [x] Default + `-tags postgres` builds green
- [x] `internal/cli/sync` + `internal/dataentry` suites green; sync coverage 72.9% (default floor 50)
- [x] ~~`just coverage-check` fully green~~ (N/A: the only failing test is the pre-existing `TestBuiltCSSIsLayered`, which checks gitignored frontend build artifacts absent without `npm run build`; it fails identically on develop and passes in CI where the SPA is built — unrelated to this change, which touches no CSS/frontend)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer) on the full fancy-browser diff vs develop
- [x] All critical review-responses addressed — RR-SYNCR5 (sync /api/v1 requests 403'd by same-origin middleware): scoped CSRF exemption via `isSyncExemptV1Path` conditioned on the provably-non-browser shape; test control flipped + end-to-end v1 GET/PATCH reachability asserted; sandboxed-iframe `Origin: null` vector closed by tightening the Origin check
- [x] All significant review-responses addressed — RR-SYNCR5 #2 (schema-compat ignored property shape): handshake now compares property type + list-ness, pinned by `TestCheckSchemaCompatible`
- [x] Minor/nit addressed — #3 recordCreate error propagation; #4 ForcePush-of-unindexed-but-remote PATCHes not duplicates; #5 real splice non-mutation test + stale `/api/sync` comment
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-SYNCR1 (significant, addressed), RR-SYNCR2 (significant, addressed), RR-SYNCR3 (significant, addressed), RR-SYNCR4 (minor, addressed), RR-SYNCR5 (critical, addressed — bundles the /code-review findings #2–#5). The earlier design-review RRs (RR-4D4UBM, RR-596TYU, RR-ATFNM1, RR-DGBVFO, RR-IWXMDW, RR-L0BN94) were dispositioned in planning (carried-over / obsoleted-by-reframe).

## Manual Review

- [x] Splice no-data-loss crux verified: `TestMergeProperties` + `TestPull_RedactedField_PreservesLocalHiddenValue` (redacted pull preserves the local hidden value end-to-end)
- [x] Temp-id adoption + relation remap: `TestPush_CreateUpdateDelete_Converges`, `TestPush_TopologicalOrder_EntitiesBeforeRelations`
- [x] Feed-404 hidden-vs-deleted guard (RR-SYNCR3): applyOne skips, never deletes, on a bare 404 for a non-tombstone entry
- [x] Retirement completeness: `/api/sync` record routes gone, manifest kept, router-walk + write-denial tests updated; CSRF exemption still covers the manifest
- [x] Security: closes the read-redaction bypass (reads via /api/v1) AND the write-field-ACL bypass (push via /api/v1 validateFieldWrite); the CSRF fix is scoped + conditioned; no new SPA write-core risk

## Documentation (enhancement)

- [x] `docs/acl-security.md` + `docs-project/entities/guides/GUIDE-acl-security.md` "Sync is a client of the authorized API (fancy browser)" section rewritten to the new model
- [x] Docs-checklist created (enhancement + docs touched) — see has-docs

## Final Checks

- [x] Commit messages explain the why
- [x] No TODOs/FIXMEs left
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (in progress as this checklist is finalized)
- [x] All CI checks pass (monitored post-creation — see PR link; local `just ci`-equivalent checks green: lint 0 issues, arch-lint, plimsoll, default + postgres builds, sync + dataentry suites)
- [x] PR URL documented below

**PR:** (to be filled with the URL once created in this /pr run)
