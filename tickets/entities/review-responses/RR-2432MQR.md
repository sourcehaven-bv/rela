---
id: RR-2432MQR
type: review-response
title: Resolve to remote lost when the guard holds the next run
finding: resolved() cleared TakeRemote on linked rows at the start of every run, so a guard-held run dropped the choice and the conflict came back.
severity: significant
resolution: The mark is cleared only when the source lists the row again, or by the gone phase; TestDeletes_ResolveRemoteSurvivesGuard.
status: addressed
---
