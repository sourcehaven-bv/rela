---
id: RR-P82DXY
type: review-response
title: grep error (exit 2) is masked into 'no assets found', misdiagnosing read failures
finding: '`grep -c ... 2>/dev/null || true` discards grep''s exit 2 (error) and collapses it with exit 1 (no match) into n=0. Verified locally: grep returns 2 on an unreadable file. The script then reports ''no embedded SPA assets found / built without running just build-frontend'', which is false and misdirects diagnosis. It is fail-closed only by accident: a non-numeric n would make `[ "$n" -eq 0 ]` error and exit non-zero under set -e. Distinguish rc>1 (scan failure) from rc==1 (genuine no-match) and report each accurately.'
severity: critical
resolution: 'Added a grep_count helper that captures grep''s exit code and distinguishes rc>1 (scan error) from rc==1 (genuine no-match), instead of `|| true` collapsing both to 0. A scan failure now reports ''FAIL: could not scan <file> (grep exit 2)'' and returns 1. Verified against a chmod-000 binary: previously it falsely claimed ''built without running just build-frontend''; it now reports the real cause. Fail-closed by design rather than by accident.'
status: addressed
---
