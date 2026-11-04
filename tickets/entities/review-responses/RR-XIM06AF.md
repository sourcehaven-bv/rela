---
id: RR-XIM06AF
type: review-response
title: Hand-off fallback for a seq no longer held
finding: "When the focused seq left the page (Jump to latest, held cap), rowFor gave the next row, so focus could jump to a far-away fold or go nowhere."
severity: minor
resolution: "The hand-off only follows a seq still in props.messages; otherwise focus is left alone, as before. Comment says so."
status: addressed
---
