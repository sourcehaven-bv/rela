---
id: RR-7UMSIM
type: review-response
title: Walk test rebuilds app + router chain per probe
finding: Each subtest constructs a fresh App and NewRouter — fine at 33 probes, slow if the table grows to hundreds.
severity: nit
reason: Full isolation per probe is more correct; cost is negligible at current scale. Revisit only if the table grows by an order of magnitude. Reviewer marked it noting-only.
status: wont-fix
---
