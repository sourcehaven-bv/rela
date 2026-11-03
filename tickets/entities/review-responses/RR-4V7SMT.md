---
id: RR-4V7SMT
type: review-response
title: Webhook report wording wrong for find-only hooks
finding: The report said every delivery creates, but a hook without create_if_missing never matches and never updates.
severity: significant
resolution: The CLI picks the wording from whether the mapping creates; docs/webhooks.md updated; find-only case added to TestACLMapWebhook_HiddenMatchFieldExitsOne.
status: addressed
---
