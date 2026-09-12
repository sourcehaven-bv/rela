---
id: RR-513JAO
type: review-response
title: Unguarded gh label create can abort the step before any issue is filed
finding: '`gh label create --force` ran bare under `set -e` before the loop. The label already exists in this repo (it deduped #993), so the call is a no-op in the normal case and its only possible effect is to kill the step on a transient 5xx — filing zero issues for every crash in that sweep. The run is already red, so the failure is indistinguishable from the sweep having failed normally.'
severity: significant
resolution: 'Appended `|| echo "WARNING: could not create/update the fuzz-failure label, continuing"`. A missing label is cosmetic; an unfiled crash is not. Verified with a stub that fails label creation: the warning prints and the issue still files, exit 0.'
status: addressed
---

Found by cranky-code-reviewer on the TKT-8LZGME diff.

```
### after fix: label create fails
WARNING: could not create/update the fuzz-failure label, continuing
  CREATE: Fuzz failure: FuzzA (internal/entity)
exit=0
```
