#!/usr/bin/env bash
# Guards the audit-log retention documentation against regressing to a
# sub-12-month cleanup example (BUG-6PYB6G / issue #887). rela never deletes
# audit logs itself; the only compliance risk is the docs suggesting an
# operator prune below the required >= 12-month window (POLICY-017 §4).
#
# Fails if docs/audit-log.md contains a `find ... -mtime +N` cleanup example
# with N < 365. The daily-rotated logs make day-granularity exact, so the
# documented example must never delete inside a year.
set -euo pipefail

DOC="docs/audit-log.md"

if [[ ! -f "$DOC" ]]; then
  echo "check-audit-retention-docs: $DOC not found" >&2
  exit 1
fi

# Extract the N from any `-mtime +N` occurrence and flag those below 365.
bad=0
while IFS= read -r n; do
  if (( n < 365 )); then
    bad=1
    echo "ERROR: $DOC documents an audit-log cleanup with -mtime +$n (< 365 days)." >&2
    echo "       Security logs must be retained >= 12 months (POLICY-017 §4); a" >&2
    echo "       shorter example would lead operators to delete records they are" >&2
    echo "       required to keep. Use -mtime +365 or longer." >&2
  fi
done < <(grep -oE '\-mtime \+[0-9]+' "$DOC" | grep -oE '[0-9]+' || true)

if (( bad )); then
  exit 1
fi

echo "check-audit-retention-docs: OK (no sub-12-month cleanup example in $DOC)"
