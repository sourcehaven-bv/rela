---
id: RR-JKZH
type: review-response
title: Misleading godoc on compareValues
finding: Says 'tries date first, then numeric, then lexicographic' but doesn't mention cross-type fallthrough. Reader assuming 'if property is date, dates are compared' will be wrong.
severity: minor
resolution: Rewrote compareValues godoc to explicitly state that type mismatch returns an error instead of falling through. Documented the strict same-type rule.
status: addressed
---
