---
id: RR-M9D9ZSG
type: review-response
title: Values hidden from the board block a name
finding: A value in the enum but left off the board columns counted as an existing column.
severity: minor
resolution: Naming such a value now adds it to the board columns only and leaves the schema alone. Tested and documented.
status: addressed
---
