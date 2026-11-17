---
id: RR-4TENE6C
type: review-response
title: 'Design: rules refuse ordinary edits without a security reason'
finding: 'set does not re-fire triggers and relation events have no producer; any fields: grant locked every set on the type; R2 ignored on.faces.'
severity: minor
resolution: 'The cascade follows create events only. A set is allowed when an editor role grants update on the type and grants the field without a condition (or declares no fields: for the type). R2 uses on.faces when present. R4 stays conservative on purpose.'
status: addressed
---
