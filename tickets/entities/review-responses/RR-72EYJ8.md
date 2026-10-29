---
id: RR-72EYJ8
type: review-response
title: Limits allow 20 buttons of up to 500 characters each
finding: 'Step 1 caps list items at 500 runes and 20 items, defaults 200 and 20. A 500-character button and 20 buttons in a message card is unusable, and the reference designs the plan cites (AskUserQuestion: 2 to 4 options) are far smaller. 20 x 500 runes x 4 bytes also exceeds the 16 KiB props cap (realtimelimits MaxMessageBytes), so the documented limit is not reachable for non-ASCII. Recommend: keep the list limits generic, but add a load warning (or refusal) under `choices.from` when the list''s `max` exceeds ~100 or `max_items` exceeds ~10, and clamp label display with CSS (full text in the accessible name).'
severity: minor
resolution: The generic list limits stay, AC 4 adds a load warning at .from above 10 items or 100 characters, labels clamp to two lines with the full text in the accessible name, and the docs note that the 16 KiB message cap can bind first.
status: addressed
---
