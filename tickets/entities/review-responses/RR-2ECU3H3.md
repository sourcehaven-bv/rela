---
id: RR-2ECU3H3
type: review-response
title: Writer interface has six methods
finding: The consumer interface exceeds the one-to-three method guidance.
severity: minor
reason: The run uses all six methods on one collaborator; splitting into two interfaces would add two Deps fields always set to the same manager without narrowing anything.
status: wont-fix
---
