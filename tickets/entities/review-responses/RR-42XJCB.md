---
id: RR-42XJCB
type: review-response
title: buildHighestIDSQL has an unchecked prefix precondition
finding: The builder required pfx to end in - and panicked on an empty string.
severity: nit
resolution: Both builders now take the bare prefix and append - and . themselves.
status: addressed
---
