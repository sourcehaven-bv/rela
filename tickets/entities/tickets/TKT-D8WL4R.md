---
id: TKT-D8WL4R
type: ticket
title: 'Triage semgrep findings from scheduled security scan (GH #1923)'
kind: chore
priority: medium
effort: s
started: "2026-10-22"
completed: "2026-10-23"
status: done
---

## Description

GitHub issue #1923: the scheduled security scan
(`.github/workflows/security-scan.yml`) reports 10 semgrep findings (p/default,
severity ERROR). Triage each: fix real issues, mark false positives with
`nosemgrep` and a specific reason.

The alerts are on the `ci/scheduled-security-scan` ref, so the default-branch
alerts API shows none; a local semgrep 1.181.0 run on develop reproduces all 10.

## Findings

- run-shell-injection: release.yml (2), security.yml (1). `${{ github.* }}` interpolated into `run:`. Fix: use the runner env vars.
- dangerous-exec-command: cmdexec.go, dataentry/commands.go. Operator-configured commands; false positive.
- pgx-sqli: docscapture/scratch_postgres.go (2), pgstore/listener.go. Quoted identifiers that are minted or constant; false positive.
- missing-unlock-before-return: lock/memlock.go, piles/kvpiles/kv.go. Unlock happens in the returned release closure; false positive.
