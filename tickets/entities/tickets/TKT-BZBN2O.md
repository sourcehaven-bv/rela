---
id: TKT-BZBN2O
type: ticket
title: 'Move state-machine when: onto predicatefns.Evaluator'
kind: chore
priority: low
effort: m
status: backlog
---

## Description

State-machine `when:` compiles in its own env
(`internal/statemachine/predicate.go` `buildEnv`): `entity` is `{id, type,
value}` plus `has_relation`/`count_relations`. Every other condition surface
uses `predicatefns.Evaluator`. TKT-205V2N bound `related()` into the existing
env instead of moving it, because the move changes `entity.value` and
`count_relations` for existing schemas and needs an arch-lint change
(statemachine may import only entity, metamodel and predicate).

Work: design a migration path for `entity.value`, move the env onto the
Evaluator, and remove the duplicate bindings.
