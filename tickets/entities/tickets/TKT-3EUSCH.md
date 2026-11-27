---
id: TKT-3EUSCH
type: ticket
title: Webhook find reads are not field-redacted by the client ceiling
kind: chore
status: wont-fix
---

## Description

A webhook find reads through the script reader (`luaWriteDeps().VisibleReader`).
In a test app with a `webhook` client baseline that has `redact: {incident:
[host]}`, `ListEntities` for the hook principal still returns `host`, and a find
on `match: [host]` still matches. docs/webhooks.md says the hook's reads have
`visible:`-hidden fields redacted.

Check whether the script reader applies client-ceiling field redaction on list
reads in production wiring. If it does not, decide whether it should, and align
the docs. `rela acl map --webhook` no longer reports a hidden match property as
a failure, because the runtime does not fail on it. Found while writing
TestWebhookCeiling_MapAgreesWithRuntime for TKT-J3RDJ4.

## Closed as wont-fix (2026-11-27)

Already fixed by BUG-KJ5D18 (9d793d968, #1956), the same root cause: a policy whose only field limits came from a client ceiling chose the no-op redactor. Pinned by the match-property case of `TestWebhookCeiling_MapAgreesWithRuntime`.
