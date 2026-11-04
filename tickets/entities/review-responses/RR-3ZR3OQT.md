---
id: RR-3ZR3OQT
type: review-response
title: '"Seen it" does nothing on a context suggestion'
finding: acknowledge() only cleared the slot and relied on the impression cooldown, which context sources ignore. The next entity change or visit brought the suggestion straight back.
severity: significant
resolution: 'New feedback kind "seen": for a context source the server snoozes it for the source cooldown; for a query source it is a no-op. TestNextAction_SeenHidesAContextSuggestion and the acknowledge tests pin it.'
status: addressed
---
