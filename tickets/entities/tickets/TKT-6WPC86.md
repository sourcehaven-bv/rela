---
id: TKT-6WPC86
type: ticket
title: rela-desktop writes a minimal schema with keys that do not exist (entity_types/relation_types)
kind: chore
priority: low
effort: xs
status: backlog
---

## Description

`cmd/rela-desktop/main.go:685-692` writes a minimal schema containing
`entity_types: {}` and `relation_types: {}`. `metamodel.Metamodel` declares
`yaml:"entities"` and `yaml:"relations"` (`internal/metamodel/types.go:33-34`),
so both keys parse to nothing.

Found during the code review of BUG-0OTGGV. It shares that bug's root cause: a
hand-rolled schema generator that bypasses `projectsetup.Initialize` and drifted
from the real format. It escaped the `id_type` defect only because it declares
no entities at all.

Prefer routing it through `projectsetup.Initialize` over correcting the key
names in place, so there is one generator rather than another copy to maintain.
