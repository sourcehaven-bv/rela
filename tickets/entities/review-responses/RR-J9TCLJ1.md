---
id: RR-J9TCLJ1
type: review-response
title: Multi-line migration error breaks validate output indentation
finding: internal/cli/validate.go prints the multi-line migration.Error after the cross mark, so the hint line is unindented.
severity: nit
reason: Pre-existing format of migration.Error, shared with the server log and rela migrate hints; not in scope.
status: wont-fix
---
