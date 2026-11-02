---
id: RR-XQ1065C
type: review-response
title: A rejected reload left the hint relay with no endpoint
finding: 'hintRelay kept one current attachment. A reload whose successor was rejected or superseded ran next.CloseAssembly, which detached the successor and left cur nil while the old assembly kept serving: sends failed and the follower followed nothing.'
severity: significant
resolution: The relay keeps its attachments in attach order; the newest still attached is current. detach removes its own entry and falls back to the one before. TestHintRelay_AttachDetachAndSwap covers attach A, attach B, detach B, and checks sends and the follower use A again.
status: addressed
---
