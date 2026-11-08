---
id: TKT-4GYJ8ER
type: ticket
title: 'rela test: execution trace, config coverage and impact analysis'
kind: enhancement
priority: medium
effort: l
status: backlog
---

## Description

Record a per-test trace: automations fired (by name), ACL decisions and the
grant that decided, transitions, validations, mail and HTTP. From it report:

- **config coverage:** which automations, grants, transitions, validations and schedules no test exercised;
- **impact analysis:** which tests depend on a given rule, so a config change runs those first.

Needs observer hooks on the automation engine and autocascade, and a recording
`acl.ACL` decorator.
