---
id: RR-IX8DIV
type: review-response
title: Pre-scan would duplicate sqlite's rules
finding: Case folding, id grammar, property marshalling, attachment caps and family type checks would be re-implemented and drift.
severity: significant
resolution: 'Plan updated: The target store judges: every row is written and every error collected; any error deletes the staging directory and lists all errors.'
status: addressed
---
