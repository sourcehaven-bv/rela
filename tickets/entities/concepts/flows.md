---
id: flows
type: concept
title: Flows
summary: Declared processes that are the only way data changes; one engine for sessions, transitions and automations
description: 'A flow declares how data changes over time: states, events, guards, steps with outputs, timers and acting roles. A run executes a flow over a subject entity.'
layer: core
status: draft
---

## Description

A flow is a declared specification of how data changes over time: states,
events, guards, steps with outputs, timers and the roles that act. A run is one
execution of a flow over a subject entity. The premise: data does not change
outside a flow; an ad-hoc edit is an implicit flow.

The engine is extracted from the realtime channel-session engine
(`internal/realtime`, `internal/realtimecfg`, `internal/channels`) and is the
single runtime for channel sessions, enum `transitions:`, `automations:` and
`flows.yaml`.
