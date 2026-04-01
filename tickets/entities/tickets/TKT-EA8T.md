---
id: TKT-EA8T
type: ticket
title: CI validation for checklist completion
kind: chore
priority: medium
effort: s
status: planning
---

## Description

Update CI to use `rela validate --check` command for validation checks instead
of the `rela analyze` commands that don't fail on errors. This ensures all
tickets have their checklists completed before merging.
