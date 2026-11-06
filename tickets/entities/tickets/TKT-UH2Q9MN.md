---
id: TKT-UH2Q9MN
type: ticket
title: Two-way peer sync through mutual pulls
kind: enhancement
priority: medium
effort: m
status: backlog
---

## Description

Two-way sync with split ownership, as two one-way pulls: each side pulls the
fields the other side owns.

- Validate at load that the two specs do not both own a field.
- Show that echoes stop: a pulled change does not bounce back.

## Acceptance

- End-to-end test with two instances that each own different fields of the same entities.
