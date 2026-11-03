---
id: TKT-CATTFF
type: ticket
title: 'Scheduled security scan: gitleaks, semgrep, osv-scanner'
kind: chore
priority: medium
effort: s
started: "2026-10-18"
completed: "2026-10-18"
status: done
description: Weekly scheduled gitleaks, semgrep and osv-scanner run that reports to code scanning and keeps one tracking issue per tool. Not on the PR path.
---

## Description

Sourcehaven ISO 27001 controls 8.28 and 8.29 ask for secret scanning, SAST and
SCA in the pipeline, with a record of the outcome. rela already has GitHub
secret scanning, CodeQL, Dependabot and the daily govulncheck in `security.yml`.
This adds one scheduled workflow that runs gitleaks (full history), semgrep
(`p/default`, severity ERROR) and osv-scanner (all lockfiles).

Like `security.yml` it does not run on the PR path: a finding in merged code is
not a defect of the PR in flight.

The repo is public, so details go to code scanning via SARIF (visible to write
access only). One tracking issue per tool carries counts and links; osv-scanner
advisories are public, so that issue lists them.

## Acceptance criteria

1. `.github/workflows/security-scan.yml` runs weekly and on `workflow_dispatch`, not on `pull_request`.
2. Results upload to code scanning per tool (categories gitleaks, semgrep, osv-scanner).
3. Per tool one issue titled "Security scan: <tool> findings" is created, updated on later runs, and closed by the first clean run.
4. Existing gitleaks false positives are listed in `.gitleaksignore` with a reason.
