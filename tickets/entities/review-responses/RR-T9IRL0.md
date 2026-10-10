---
id: RR-T9IRL0
type: review-response
title: Escape mid-drag then release drills
finding: cancel() did not suppress the click that follows the still-held press, so Escape followed by release navigated into the node.
severity: significant
resolution: cancel() of a moved gesture swallows the next click until the press ends (window pointerup once). The Escape test now asserts the click is swallowed.
status: addressed
---
