---
id: RR-U2ESX0P
type: review-response
title: A failed first-row read leaves the step blank
finding: Any firstQuery error kept the state at loading with an empty div.
severity: minor
resolution: 'New error state with an en/nl message. Vitest case: says so when the rows cannot be read.'
status: addressed
---
