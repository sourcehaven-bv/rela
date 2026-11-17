---
id: RR-14Y1PQR
type: review-response
title: Rules that can never fire still load
finding: A rule with no event key, or with no outputs and no checks, loads and never fires; check.severity is not validated.
severity: nit
resolution: Check refuses a rule with no event key / a rule with neither outputs nor checks / an unknown check severity; all migrated projects still load.
status: addressed
---
