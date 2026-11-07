---
id: TKT-YWX30D6
type: ticket
title: Check the asserted org before storing IdP roles
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

The IdP webhook stores the asserted org id with each role record (TKT-IZL6NX8)
but never compares it to the deployment org. With a shared issuer and audience,
a webhook for one org could be replayed to another. Enforce the org once the R1
org design lands; until then the guide tells operators to use a unique
-webhook-audience per deployment. Deferred from RR-1A4C66C.
