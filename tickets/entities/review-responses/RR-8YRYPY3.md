---
id: RR-8YRYPY3
type: review-response
title: Trigger legacy clause does not bind date variables
finding: flowcfg/triggers matchClause runs filter.Match on a legacy clause without binding date variables.
severity: minor
reason: Triggers run on the write path; this ticket covers read-time surfaces.
status: deferred
---
