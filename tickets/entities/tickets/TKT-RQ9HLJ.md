---
id: TKT-RQ9HLJ
type: ticket
title: 'IdP webhook receiver principal is not under the reserved system: prefix'
kind: chore
priority: medium
effort: s
status: backlog
---

## Description

The IdP webhook receiver (`/webhooks/idp`, internal/dataentry/webhook.go) stamps
its principal as `webhook:<event>`, without the reserved `system:` prefix.
`principal.IsReserved` does not cover that name, so a deployment that trusts a
principal header lets a caller assert `webhook:<event>` on the ordinary API and
inherit any grant acl.yaml gives that name.

Move the receiver's identity under `system:` (for example
`system:idp-webhook:<event>`), and give it a principal_type if it should be
cappable. Found during the design review of TKT-J3RDJ4.
