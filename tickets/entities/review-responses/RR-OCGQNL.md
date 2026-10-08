---
id: RR-OCGQNL
type: review-response
title: Long endpoints only partly checked but weighted in full
finding: Only the 512 runes next to the block boundary were aligned while the endpoint counted at full length, so a paragraph whose first 1300 runes were replaced still resolved.
severity: significant
resolution: The far edge of a long endpoint is aligned at its predicted position and combined; each endpoint is weighted by the runes compared. Pinned by 'most of a long endpoint rewritten' and TestResolveCrossBlockLongEndpointEdited.
status: addressed
---
