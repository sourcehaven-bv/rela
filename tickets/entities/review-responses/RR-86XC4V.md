---
id: RR-86XC4V
type: review-response
title: ChangeSource placement and arch-lint statement
finding: 'Architecture design review: Deps.Changes is acceptable because reload reuses the store, but the plan should read it once per watch, say why the interface repeats store.Watcher''s method set, and correct the reason for not using changefeed.'
severity: minor
resolution: The plan reads Deps.Changes once per watch, explains the consumer-side declaration, and gives the real reason changefeed is not used.
status: addressed
---
