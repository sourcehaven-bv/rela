---
id: RR-2FW567B
type: review-response
title: 'Validation where: did not bind date variables'
finding: 'The relation-constraint where: and the untranspilable fallback in validation ignored $today.'
severity: significant
resolution: Both bind date variables at the evaluator's read time (Evaluator.ReadTime).
status: addressed
---
