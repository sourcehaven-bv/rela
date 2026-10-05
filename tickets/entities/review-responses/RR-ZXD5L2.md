---
id: RR-ZXD5L2
type: review-response
title: deleteTarget prompt row is non-deterministic
finding: deleteTarget returned family[0] from an unordered listing.
severity: nit
resolution: It now picks the lowest face so the prompt is stable.
status: addressed
---
