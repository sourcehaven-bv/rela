---
id: RR-YBXQ24
type: review-response
title: Channel budget tests do not pin the hidden-ref path or channel store calls
finding: No ref is unreadable and the channel store is not counted, so a per-row lookup on the restricted branch or a per-message store read passes.
severity: significant
resolution: Half the refs point at unreadable entities and render only restricted; a counting channel store and entity store pin per-method calls identical at 10 and 50.
status: addressed
---
