---
id: RR-8P25KJ
type: review-response
title: Source allowlist belongs on the source, not on N views
finding: 'CommandScope and visible_worlds scope placement on the item being placed; per-view sources: lists force editing every view to place a source.'
severity: significant
resolution: 'Planned: source gains visible_on: [surface or surface:view]; views get only a suggestions: bool. Precedence: view bool, then surface bool, then enabled.'
status: addressed
---
