---
id: RR-ABGOZ1
type: review-response
title: Server-derived busy has a precedent the plan dismisses
finding: The alternative 'derive busy on the server from list_id' is rejected because the server would need two ways to decide the busy type. listRollups (internal/dataentry/rollups.go:140-160) already derives per-row extras from list_id and the list's columns in serveListRowExtras (comment_counts.go:44). The chosen client-side param is fine, but the stated reason is weak. Say instead that the board sends busy explicitly and one mechanism keeps the live path and the request identical.
severity: nit
resolution: ADDRESSED the server-derived alternative now cites listRollups and serveListRowExtras and is rejected because one explicit parameter keeps board, list and live path identical
status: addressed
---
