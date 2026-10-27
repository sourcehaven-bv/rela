---
id: RR-EJB3GQ
type: review-response
title: History responses must not carry roll-ups
finding: A current roll-up on an old snapshot is wrong and the field scope errors for history subjects.
severity: minor
resolution: Roll-ups are merged only on live entity GET, write responses, list rows, view sections and MCP show_entity; history/version handlers are untouched.
status: addressed
---
