---
id: RR-QJABHE
type: review-response
title: 'Document the interaction with when_closed: post'
finding: 'A trigger with when_closed: post sets requireOpen=false (trigger.go:1274) and may post into a closed channel. With in:, such a post is now skipped when the session''s final state is not in the type''s in:. Add one sentence next to when_closed in docs/realtime.md.'
severity: nit
resolution: 'AC 6, Q4, the test plan and the docs list now cover a `when_closed: post` trigger that is skipped when the closed channel''s final state is outside `in:`.'
status: addressed
---
