---
id: RR-F5WMM33
type: review-response
title: Flow immutability leaks through Branch.Roles and Guard.Fields
finding: clone shared Roles and the exported Guard fields.
severity: minor
resolution: clone copies Roles; Guard fields are unexported behind accessors, Fields returns a copy.
status: addressed
---
