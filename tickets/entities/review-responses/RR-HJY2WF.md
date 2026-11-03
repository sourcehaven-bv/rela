---
id: RR-HJY2WF
type: review-response
title: 'Side panel swallows traversal errors'
finding: 'executeSidePanel returned nil on any error without logging; a bad where: now errors instead of widening.'
severity: minor
resolution: 'Logs a slog.Warn with the entity and error.'
status: addressed
---
