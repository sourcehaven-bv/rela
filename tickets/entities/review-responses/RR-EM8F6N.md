---
id: RR-EM8F6N
type: review-response
title: 'PR 8 security: zero selection renders as the default world'
finding: faceSelectionCond fell through to face = '' for a zero selection.
severity: minor
resolution: Zero selection renders false on pg and 0 on sqlite (fail closed).
status: addressed
---
