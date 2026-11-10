---
id: tenant-webhook-replay-after-eviction
type: automated-measure
title: A replayed IdP webhook is rejected after its tenant was evicted and reopened
description: A tenant-mode test delivers a webhook, evicts and reopens the tenant, delivers the same webhook again and asserts the replay is rejected.
kind: test
location: internal/dataentry or internal/tenanthost (webhook replay test)
status: proposed
---

Fails when the webhook dedup set is kept per App instead of in the tenant's
state, so an eviction forgets which webhooks were seen.
