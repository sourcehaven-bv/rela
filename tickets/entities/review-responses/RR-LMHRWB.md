---
id: RR-LMHRWB
type: review-response
title: Nil traversal binders pass silently
finding: WithTraversals and validator.New accepting nil fail per rule at run time.
severity: minor
resolution: validator.New takes a required binder and rejects nil; each site passes an Ungated-backed or refusing binder explicitly.
status: addressed
---
