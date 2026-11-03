---
id: TKT-3EUSCH
type: ticket
title: Webhook find reads are not field-redacted by the client ceiling
kind: chore
status: backlog
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
