---
id: RR-I57P0
type: review-response
title: security.yml pipeline relies on implicit pipefail; make it explicit
finding: '`./scripts/govulncheck-filtered.sh 2>&1 | tee /tmp/vulncheck.log` relies on GitHub''s default `bash -eo pipefail`. If anyone ever sets `shell: bash` without flags, tee''s success masks the script''s failure and the `if: failure()` step never runs. Fix: add explicit `set -o pipefail` in the step, or drop the pipe in favour of `>file 2>&1; status=$?; cat file; exit $status`.'
severity: critical
resolution: security.yml govulncheck step now uses explicit `set -euo pipefail` and writes to a temp file with `>/tmp/vulncheck.log 2>&1 || status=$?; cat; exit $status` — no pipe, no dependency on GitHub's default shell flags.
status: addressed
---
