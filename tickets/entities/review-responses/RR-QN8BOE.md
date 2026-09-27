---
id: RR-QN8BOE
type: review-response
title: NewBinder call sites pass store methods without a nil check
finding: Call sites built binders from st.MatchingIDs directly; a nil store panicked late instead of failing at construction.
severity: minor
resolution: relresolve.NewStoreBinder rejects a nil store; all call sites use it.
status: addressed
---
