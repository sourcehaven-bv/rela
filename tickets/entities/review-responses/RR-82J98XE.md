---
id: RR-82J98XE
type: review-response
title: The timeline source differs from the ticket
finding: The ticket says the timeline comes from the audit log.
severity: minor
resolution: 'Recorded as a decision in the ticket: the audit sink has no reader and header history depends on the backend (fs has none), so active runs keep a bounded timeline in their state; finished runs show start and end.'
status: addressed
---
