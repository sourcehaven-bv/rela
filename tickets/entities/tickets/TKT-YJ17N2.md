---
id: TKT-YJ17N2
type: ticket
title: Guard test forbids zero-face reads outside a shrinking allowlist; faced fixtures by default
kind: enhancement
priority: high
effort: m
status: done
description: 'Stage 0 of RES-Y6JA37: a go/ast guard test pins today''s zero-face read call sites and fails on new ones; tests use realistic faced fixtures.'
---

## Description

Stage 0 of RES-Y6JA37 / DEC-NPZICR. Stop new zero-face reads from being written
while the model changes.

- A `go/ast` guard test (precedent: `dataentry/world_test.go:367`, `acl/ceilingguard_test.go`) that fails on a call to `store.Store.GetEntity`, `entityReader.getEntity` or `bareEntityID` outside an allowlist. The allowlist pins today's call sites (file and count) and may only shrink; a new call site fails with a message pointing at the address-aware alternatives.
- Make the realistic faced fixture (`facedApp`, `seedDeclaredFaceTicket`) the default in `internal/dataentry` tests. Retire `seedDraftAndPublishedTicket`, which seeds a legacy zero-face row beside the faced rows and so lets zero-face reads pass.

## Acceptance

- Adding a new `GetEntity(ctx, id)` call in a non-allowlisted file fails the guard test.
- Removing an allowlisted call without shrinking the allowlist fails too, so the list stays exact.
- No test seeds a zero-face row for a type that declares faces.
