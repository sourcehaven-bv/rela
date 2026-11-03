---
id: RR-68MJGU
type: review-response
title: One checker entry can stop the project loading
finding: Declaring checkers makes the load fail when any type name has a space or slash.
severity: minor
reason: 'Intended: a silently skipped type would look checked. The error names the type and the pattern, and docs/checkers.md documents it.'
status: wont-fix
---
