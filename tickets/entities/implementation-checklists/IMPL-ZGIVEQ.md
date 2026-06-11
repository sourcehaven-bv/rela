---
id: IMPL-ZGIVEQ
type: implementation-checklist
title: 'Implementation: Weekly fuzz sweep over all targets with auto-filed issues'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — committed regression seeds (4 control-char inputs in memstore/fsstore testdata); script failure paths verified by injection
- [x] Integration tests written — the sweep itself ran locally end-to-end
- [x] Happy path implemented (script, workflow, recipe)
- [x] Edge cases handled (bad FUZZTIME → exit 2; broken build → gate, exit 2; zero targets → exit 2; failure classification fuzz-crash vs error; issue dedupe; setup errors don't file issues)
- [x] Error handling in place

## Test Quality

- [x] Discovery-based (no stale hand-list) — same loud-failure principle as the TKT-TLQ94B inventory tests
- [x] Oracle delegated to the shared production contract (storeutil.ValidateID/ValidateRelationType/ValidateProperty), directional, with documented vacuity anchor
- [x] No hardcoded values in assertions — n/a
- [x] Only specifying what matters — n/a
- [x] Property comparisons — n/a

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence:**

- First local 2s sweep over all 39 discovered targets immediately found **5 failures**: 4 stale fuzz-harness oracles (storetest collision harnesses hand-modeled ID validity, went stale when control-char rejection was added) and **1 real production bug** (GenerateShortID emits validator-rejected IDs for pathological prefixes → BUG-RHFHTH filed with full repro; its crashing input deliberately not committed).
- Harness fix: directional oracle via `createEntityOrSkip` + new `storeutil.ValidateRelationType` used by all three backends (also fixes pgstore feed.go's documented-but-unenforced control-char assumption and fsstore's `/`-in-relType directory hazard). The 4 crash inputs are now committed regression seeds — verified passing on both backends.
- Re-fuzz after fixes: all four collision targets clean at 6–8s budgets; full store packages + entitymanager + dataentry green under `-race`; lint 0 issues.
- Failure-path verification: deliberate failing target → recorded + exit 1; `FUZZTIME=banana` → exit 2 without issue-summary; classification verified (`fuzz-crash` tag on the known-red GenerateShortID target).

## Quality

- [x] Code follows project patterns (scripts/ precedent, security.yml cron pattern, minimal workflow permissions)
- [x] DRY — oracle helper, shared validator
- [x] No security issues (no untrusted input in workflow run blocks; --body-file for issue content)
- [x] No silent failures (exit-code split: 1 = findings, 2 = setup; issues filed only for findings)
- [x] No debug code left behind
