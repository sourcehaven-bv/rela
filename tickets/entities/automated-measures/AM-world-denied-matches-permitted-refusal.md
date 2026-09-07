---
id: AM-world-denied-matches-permitted-refusal
type: automated-measure
title: A non-world-capable route answers a denied world exactly as it answers a permitted one
description: On a route outside worldCapablePath's allowlist, an explicit ?world=<declared> must produce the same status and body whether or not the principal holds the read grant for that world. Pins BUG-CV8L3B, where a denied world short-circuited past the route-capability refusal and received full default-world content while a permitted world got 422.
kind: test
location: internal/dataentry/world_test.go
status: proposed
---

On a route outside `worldCapablePath`'s allowlist, an explicit
`?world=<declared>` must produce the **same status and body** whether or not the
principal holds the read grant for that world.

## Why this shape

The obvious assertion — "a denied world must not return default-world content" —
catches only the disclosure. This one also catches the **grant oracle**: any
future divergence between the permitted and denied responses fails it, including
divergences that leak nothing but the grant bit itself.

It is the property `resolveWorld` already claims in prose ("indistinguishable
from a world with nothing in it, by design"), asserted against the *refusal*
path rather than only the empty-result path.

## Realized by

`TestAttachWorld_DeniedWorldRefusedLikePermitted` in
`internal/dataentry/world_test.go`, table-driven over the refused routes
(`_analyze`, `_documents/…`, entity sub-resources), comparing the two recorders
field by field.
