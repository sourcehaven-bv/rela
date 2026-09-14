---
id: RR-EAW6EL
type: review-response
title: Last summary row dropped when file lacks a trailing newline
finding: '`while read -r` returns non-zero on a final line with no trailing newline and bash discards it, and the `sed` in the process substitution does not add one. A `fuzz-failures.txt` written without a terminating newline silently loses its LAST row at exit 0 — and that is not a random sample, it is always the alphabetically-last target, so the same finding would vanish every week. Today `fuzz-all.sh` uses `echo` so every row is terminated, but that is a property of the producer, not a guarantee the consumer checks; the two files sit in different directories with no test between them.'
severity: critical
resolution: The process substitution now appends a newline (`sed ...; printf '\n'`), so a final partial line is always terminated before `read` sees it. The existing `[ -n "$target" ] || continue` guard skips the resulting blank row, verified separately. Reproduced before the fix (a printf-written 2-row file filed only FuzzA) and verified after (both FuzzA and FuzzB file, exit 0).
status: addressed
---

Found by cranky-code-reviewer on the TKT-8LZGME diff.

```
### before fix: no trailing newline
  FILED: Fuzz failure: FuzzA (internal/entity)
  (FuzzB never processed, exit 0)

### after fix
  CREATE: Fuzz failure: FuzzA (internal/entity)
  CREATE: Fuzz failure: FuzzB (internal/store)
exit=0
```

Also checked the newline does not itself create a junk issue: a summary ending
in two blank lines files exactly one issue.

The `sed` was anchored at the same time (`s/ \[([^]]*)\]$/ \1/`) so it strips
only the trailing kind field rather than the first bracket group anywhere on the
line (reviewer finding 9).
