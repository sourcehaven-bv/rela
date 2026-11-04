---
id: RR-NFXMQEM
type: review-response
title: focusDecisions relies on DOM order for choice before decision
finding: RlChannel.vue focusDecisions queried .rl-channel-decisions and took the first in DOM order.
severity: minor
resolution: It queries .rl-channel-message__choices first, then .rl-channel-decisions.
status: addressed
---
