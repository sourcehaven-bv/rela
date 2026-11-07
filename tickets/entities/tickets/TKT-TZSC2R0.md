---
id: TKT-TZSC2R0
type: ticket
title: Opt-in maximum age for stored IdP roles
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Stored IdP roles (TKT-IZL6NX8) last until the next webhook for the user. A lost
delivery leaves a revoked role in force for background jobs. Add an opt-in
maximum age after which a stored record grants nothing, plus guidance on
resyncing before turning it on. Deferred from RR-PX6L3P5.
