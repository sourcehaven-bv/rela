---
id: RR-48VXPVJ
type: review-response
title: Truncation failed in editable detail rows
finding: Inside the inline editor the value had no width limit, so the ellipsis never showed.
severity: significant
resolution: The link is capped at 40ch; measured in Chromium in the inline editor.
status: addressed
---
