---
id: RR-D83PLN6
type: review-response
title: Cancel decision leaves a hidden reply
finding: Other answer and onAnswer set the reply; cancel picks a default from a reply-filtered list, which may be empty, leaving replyTo set with no strip.
severity: significant
resolution: 'Plan 2: the form decision records startedReply; cancelCompose ends that reply. Safeguard keeps the composer from vanishing. Test: Other answer then Cancel ends the reply (AC 4).'
status: addressed
---
