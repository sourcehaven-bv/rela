---
id: BUG-S6S1Z0
type: bug
title: IdP webhook principal is outside the reserved prefix
description: IdP webhook actions run as webhook:<event> which is outside the reserved prefix. A request can therefore assume that principal.
priority: high
effort: s
status: backlog
---

## Description

IdP webhook actions run as `webhook:<event>`
(internal/dataentry/webhook.go:180). That name is not under `system:` or
`integration:`, so `principal.IsReserved` does not refuse it at the request
boundary. A request that asserts it through `-principal-header` inherits
whatever grants an operator gave that principal in acl.yaml.

## Fix

Use a name under the reserved prefix (for example `system:webhook:idp:<event>`)
and keep accepting the old name in acl.yaml with a load warning.

Found during TKT-NXRFDU.
