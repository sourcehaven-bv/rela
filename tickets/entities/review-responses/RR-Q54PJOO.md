---
id: RR-Q54PJOO
type: review-response
title: No test pins Flow() against the compiled session
finding: withFlow duplicates the compiled states and reminders; nothing checks the copy matches.
severity: significant
resolution: Added TestSession_FlowMirrorsCompiled in realtimecfg.
status: addressed
---
