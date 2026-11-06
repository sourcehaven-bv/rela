---
id: BUG-FQZ5Q1L
type: bug
title: Relation move skips the manager's relation meta gate
description: A relation position move writes the order meta key without the field-gated manager's relation meta check; only the dataentry handler checks it.
priority: medium
effort: s
why1: Manager.UpdateRelation gates opts.Properties through checkRelationMetaUpdate; a move carries no properties so the gate returns at once and moveRelation writes the order key unchecked.
why2: moveRelation computes the order values itself inside its Tx and writes them straight to the store; it was written before the manager had a relation meta gate.
why3: BUG-O8QP7GX added the manager gate to the property paths of CreateRelation and UpdateRelation and judged the move covered because the data-entry handler checks the order key.
why4: The handler check and the manager check are two layers; tests for the move stubbed the handler's verdicts and never drove a field-gated manager with a Position.
why5: Write paths that derive the values they store (moves and renumbers) are not listed next to the gate they must pass so a new gate is wired only into the paths that take caller values.
prevention: 'TestMoveRelation_FieldGateJudgesOrderKey drives a field-gated manager with outgoing and incoming moves; TestRenumberAfterUpdate_FieldGateJudgesSiblingSources pins the renumber after an explicit order write; TestRelationPosition_FieldsGrantRealPolicy pins the handler against a compiled fields: policy. BUG-T0MFVJG tracks one rule for every order-preserving rewrite of other sources'' edges.'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

## Description

A relation PATCH with `position` moves an edge among its siblings and writes the
order meta key (`_order_out` / `_order_in`). The dataentry handler checks that
key against the relation verdict before it calls the manager. The manager itself
does not: `Manager.UpdateRelation` runs `checkRelationMetaUpdate` on
`opts.Properties`, which is empty for a move, and `moveRelation` then writes the
order key, and on the incoming side the order key of other sources' edges.

The field-gated manager is the write-path backstop for every surface
(BUG-O8QP7GX). A move that reaches the manager without the handler's check
writes a key the policy may refuse.

Follow-up to PR #2101.
