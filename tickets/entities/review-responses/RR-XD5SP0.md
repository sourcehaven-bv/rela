---
id: RR-XD5SP0
type: review-response
title: Guard env should declare reminder names that fail other checks
finding: 'Plan lines 421-422 compile reminders before transitions so the guard env knows their names. If a reminder with a bad every or post is left out, as compileTimer drops a timer with problems, every guard that names it gets a second "unknown attribute" error. Plan change: declare every reminder whose name matches the grammar, whatever its other problems.'
severity: nit
resolution: 'Addressed: every reminder whose name matches the grammar is declared in the guard env whatever its other problems, so a guard naming it reports no second error. Row in AC 4.'
status: addressed
---
