---
id: RR-XUL7ROY
type: review-response
title: Fallback channel type in InboxWaitingItem is a force-cast placeholder
finding: The ct computed uses as unknown as ChannelTypeSchema; it is never used for rendering because every button is gated on held, but the cast is unexplained.
severity: nit
resolution: 'Kept the cast with a comment: the stand-in reaches only decisionViews and choiceViews, which read session and message_types, and every button is gated on held.'
status: addressed
---
