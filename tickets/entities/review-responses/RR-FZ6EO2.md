---
id: RR-FZ6EO2
type: review-response
title: Fold text needs a plural-aware message function
finding: '''1 more message'' vs ''3 more messages'' and the label list need a function message like channelNewMessages, not a ChannelTextKey entry (useMessages.ts:74, 395). Say so in plan step 5 so the nl plural is handled.'
severity: nit
resolution: Step 5 now specifies a plural-aware message function channelFold({ count, labels }) like channelNewMessages, and the other count text channelOlderHidden follows the same pattern.
status: addressed
---
