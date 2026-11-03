---
id: RR-2BXYO9
type: review-response
title: presentationView passes an unknown display value through as is
finding: 'channelView.ts:77 copies mp.display without checking it. If a newer server sends a third value, quietDisplay treats it as quiet without being collapsed, so the messages are silently hidden. Fix: copy only ''collapsed'' and ''hidden'', and refer to the library''s ChannelMessageDisplay type in frontend/src/types/config.ts instead of repeating the union.'
severity: nit
resolution: Fixed. presentationView copies display only when it is collapsed or hidden, so an unknown value shows the messages in full; new channelView case. frontend/src/types/config.ts uses the library's ChannelMessageDisplay type.
status: addressed
---
