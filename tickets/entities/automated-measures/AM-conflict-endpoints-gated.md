---
id: AM-conflict-endpoints-gated
type: automated-measure
title: 'Test: conflict list and detail endpoints honour the read gate'
description: 'Planned for BUG-Q3Z15V. Seeds a conflicted file for an entity the principal cannot read and asserts it is absent from GET /_conflicts and a uniform 404 on GET /_conflicts/{path}, and that visible: fields are redacted on both sides.'
kind: test
location: internal/dataentry (planned)
status: proposed
---
