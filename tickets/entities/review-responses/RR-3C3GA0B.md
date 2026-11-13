---
id: RR-3C3GA0B
type: review-response
title: Session storage changes missing from the plan
finding: SQL backends store the session in columns with migrations; the file backend in meta.yaml; flow.Instance is also stored by flows with Check/Norm knowing only current fields.
severity: significant
resolution: 'Plan: asked lives on channels.Channel (not flow.Instance), cleared by ApplySession on a move. pg and sqlite migrations, a meta.yaml field, and a cap check on read and write like CheckReminders. Flows untouched.'
status: addressed
---
