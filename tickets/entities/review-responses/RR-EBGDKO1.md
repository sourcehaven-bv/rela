---
id: RR-EBGDKO1
type: review-response
title: Blank line before return in triggerProblems
finding: configedit/triggers.go:49-50 has an empty line before return nil inside the condition block.
severity: nit
reason: golangci-lint's whitespace rule requires the blank line after a multi-line if condition.
status: wont-fix
---
