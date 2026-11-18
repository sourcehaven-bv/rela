---
id: RR-N2SO2E3
type: review-response
title: 'Flows ignore scope grant visible:'
finding: triggeracl checks ScopeGrants on top of FieldRestricted; flowcfg does not.
severity: minor
reason: 'Scope grants only reopen fields on top of a client baseline visible: (ceilingcompile.go), so they cannot hide a field the baseline shows; the baseline is already checked. Out of scope.'
status: deferred
---
