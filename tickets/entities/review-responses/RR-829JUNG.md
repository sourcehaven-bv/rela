---
id: RR-829JUNG
type: review-response
title: localContent ?? '' fallback merges from an empty base
finding: 'from: sent.localContent ?? '''' would treat the whole text as added on both sides.'
severity: minor
resolution: The rebase is set only when localContent is defined; otherwise the base moves as before.
status: addressed
---
