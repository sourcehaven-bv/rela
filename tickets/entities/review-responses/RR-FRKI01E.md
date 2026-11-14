---
id: RR-FRKI01E
type: review-response
title: XML text copied into every ancestor
finding: Each element stored its full inner text, so memory grew with depth times size.
severity: significant
resolution: The node keeps its safexml element and computes inner text only when a path or the hook reads it.
status: addressed
---
