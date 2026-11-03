---
id: RR-BZZ0FV
type: review-response
title: Per-page re-resolve burns suggestions through cooldown
finding: markShown starts a 24h cooldown and eligibleFromSource drops keys in cooldown, so re-resolving on every navigation shows a different suggestion per click and a context hint vanishes on revisit.
severity: critical
resolution: 'Planned: GET carries current=<source,entity,variant>; the engine keeps that key exempt from cooldown while it is still eligible and no higher band fires. Context sources are not suppressed by impression cooldown, only by snooze, dismiss and mute. Test A->B->C keeps one suggestion.'
status: addressed
---
