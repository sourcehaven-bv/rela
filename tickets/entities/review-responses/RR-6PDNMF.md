---
id: RR-6PDNMF
type: review-response
title: Legacy arming is inconsistent across generations and rewrites the displayed entered_at
finding: 'Arming shields only generation-0 legacy sessions: their from: state timers count from the arm time. A slice-1 session at generation >= 1 counts from its real EnteredAt and fires at once when a timer is added, as does any session added after this ships. Arming also changes the wire entered_at of a never-moved session from created_at to the arm time (currentSession, internal/realtime/session.go:60), which turns a historical fact into the deploy date and shifts when: guards that read session.entered_at. Plan change: pick one rule. The simplest is to drop started_at and arming: from: session counts from CreatedAt and the documented reload rule (''adding a timer can fire it'') applies everywhere. That removes a column, the arm writes and the rev special case. If started_at must stay (a session block added to an existing type), keep entered_at honest on the wire and use max(EnteredAt, StartedAt) only inside timerDue.'
severity: minor
resolution: 'Plan updated: started_at and arming dropped; since: session counts from created_at; entered_at stays as in slice 1; the reload rule (adding a timer can fire it) applies to all sessions.'
status: addressed
---
