---
id: TKT-LM1EOUI
type: ticket
title: Record the properties an on-create automation sets
kind: enhancement
priority: low
effort: s
status: backlog
---

## Description

`writeAutomationProperties` (internal/entitymanager/cas.go) writes the
properties an on-create automation sets after the create, and no audit record
describes that write. The create record shows the entity as the caller sent it.
Record the follow-up write in its own transaction, as every other write now does
(TKT-TU0VZ4Y), or fold the properties into the create record.
