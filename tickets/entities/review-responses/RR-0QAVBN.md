---
id: RR-0QAVBN
type: review-response
title: Item trimming differs from scalar strings
finding: Scalar string props are not trimmed (internal/realtime/validate.go:125-131); list items are. Fine for options, but document it in docs/realtime.md next to the list property row so authors are not surprised.
severity: nit
resolution: Step 4 and the docs plan note that list items are trimmed while scalar strings are not.
status: addressed
---
