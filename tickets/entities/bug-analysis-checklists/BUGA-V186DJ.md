---
id: BUGA-V186DJ
type: bug-analysis-checklist
title: 'Analysis: govulncheck auto-update picks wrong Go minor and duplicates issues'
started: "2026-10-25"
completed: "2026-10-25"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Run 40179808424 (2026-10-25): fixable.sh printed `stdlib 1.26.10` and `stdlib
1.27.3`; the workflow ran `go get toolchain@go1.27.3`; the re-scan failed to
load go1.27 sources ("file requires newer Go version go1.27 (application built
with go1.26)"), so it filed #1986. Locally the old script on a two-branch OSV
fixture prints both lines (scripts/govulncheck-fixable-test.sh fails 3 of 7
against it). Issue timeline for #1986 has no `labeled` events, while #1922
(GITHUB_TOKEN) has them; `gh issue list --label security,automated` therefore
returns nothing.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

Fix: use `finding.fixed_version` and `trace[0].module`, highest per module by
version sort; file the issue with GITHUB_TOKEN and dedup on title only; keep
govulncheck stderr out of the JSON. Test: scripts/govulncheck-fixable-test.sh,
run in the CI Demos job. security-scan.yml already uses GITHUB_TOKEN for its
issues, so it is not affected.
