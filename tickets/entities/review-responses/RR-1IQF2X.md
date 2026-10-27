---
id: RR-1IQF2X
type: review-response
title: Background automation attaching to its own trigger re-triggers itself
finding: The attachment stamp is a save which re-enqueues the save automation.
severity: significant
resolution: 'Plan: guide documents the loop and recommends a when: guard; test an on-save background automation that attaches to its own entity with a guard and asserts one render.'
status: addressed
---
