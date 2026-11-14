---
id: RR-QLYXN2S
type: review-response
title: Scope drift creates duplicates
finding: An entity whose scope property was edited falls out of matching and would be created again.
severity: minor
resolution: A key found on a row of the type outside the scope is a record error, not a create.
status: addressed
---
