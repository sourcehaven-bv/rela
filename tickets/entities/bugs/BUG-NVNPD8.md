---
id: BUG-NVNPD8
type: bug
title: govulncheck auto-update picks wrong Go minor and duplicates issues
description: 'Daily govulncheck (GO-2026-6809; GitHub #1945 #1953 #1962 #1970 #1986) filed a new issue each day instead of an auto-merging go1.26.10 toolchain PR.'
priority: high
effort: s
why1: The auto-update ran go get toolchain@go1.27.3 instead of go1.26.10; the re-scan then failed to load packages so the workflow filed an issue. Separately the issue-dedup query filtered on labels the issues do not carry.
why2: govulncheck-fixable.sh emitted every fixed event in the OSV ranges (one per Go release branch) and the workflow kept the lexicographically last per module. The issues were created with the GitHub App token and GitHub silently drops labels the creator may not set.
why3: The script read raw OSV ranges instead of the finding's fixed_version that govulncheck computes for the version in use. The dedup relied on labels without checking they were applied.
why4: '#1824 added the stdlib branch but was verified against an advisory with a single fixed event; no test fed the script a multi-branch OSV. The issue step was never observed on a second run.'
why5: The security workflow's own logic has no tests and only runs on schedule; its failure modes surface only as noise in production issues.
prevention: Test the fix picker in CI (scripts/govulncheck-fixable-test.sh with a multi-branch OSV fixture). Derive the fix version from govulncheck's per-finding fixed_version instead of raw OSV ranges. File issues with GITHUB_TOKEN so labels apply and dedup on title so unlabeled issues still match.
started: "2026-10-25"
completed: "2026-10-25"
status: done
---

## Description

GitHub #1945, #1953, #1962, #1970 and #1986. The daily govulncheck scan reports
GO-2026-6809 (crypto/x509), fixed in go1.26.10. Each run opened a new issue, and
none opened the auto-merging toolchain-bump PR.

## Root causes

1. Auto-update picked the wrong fix version. `govulncheck-fixable.sh` read
every `fixed` event of the OSV ranges. GO-2026-6809 has one per release branch
(1.26.10 and 1.27.3). The workflow kept the last line, which sorts
lexicographically, so it ran `go get toolchain@go1.27.3`. govulncheck, built
with go1.26, cannot load the go1.27 standard library, so the re-scan failed and
the workflow filed an issue.
2. Dedup never matched. The issue is created with the GitHub App token, and
GitHub drops labels on a new issue when the creator may not set them. The issues
have no labels, but the dedup query filters on `security,automated`, so it finds
nothing and creates another issue.

## Fix

- go.mod `toolchain go1.26.10` (all workflows read go.mod).
- `govulncheck-fixable.sh` uses each finding's `fixed_version`, which
govulncheck computes for the version in use, and picks the highest per module
with a version sort. A test script pins this.
- The issue step uses `GITHUB_TOKEN` (labels apply, as in security-scan.yml)
and dedups on the exact title only.
