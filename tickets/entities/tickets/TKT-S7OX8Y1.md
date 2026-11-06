---
id: TKT-S7OX8Y1
type: ticket
title: Move resolved view payload types into apiwire/v1
kind: refactor
priority: medium
effort: m
status: backlog
---

## Description

Several resolved payloads have no type in `internal/apiwire/v1`: `_views`
sections, dashboard cards and next-action bands. Move their wire types there so
they are generated like the rest, with no change to the JSON.

Part of FEAT-QRI3CFR (step 0.4).
