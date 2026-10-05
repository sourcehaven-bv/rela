---
id: TKT-LTV29O
type: ticket
title: 'CalDAV writes: apply field and relation affordances'
kind: chore
status: backlog
---

## Description

CalDAV writes (`internal/dataentry/caldav_write.go`, `attachToDriver` /
`unlinkFromDriver`, plus field writes from a calendar client) apply the ACL and
the read gates but no affordance rules: no field affordances, no relation
affordance gate (`relationSources` / `relationOpDenial`). A `when:` rule or a
read-only field that the web app enforces can be bypassed from a calendar
client.

## Acceptance

- CalDAV membership edges run the same relation affordance gate as the web
app's relation writes, including the family rule for identity-scoped edges from
a faced entity.
- CalDAV field writes run the field affordance gate.
- A refusal maps to a CalDAV-appropriate status (403) and names no hidden face.
