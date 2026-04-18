---
id: TKT-CO4YP
type: ticket
title: Tighten storeutil.ValidateID and add storetest.Capabilities
kind: refactor
priority: medium
effort: xs
status: ready
---

## Description

Preparatory work for an upcoming boltstore backend. Two changes to the shared
store test-kit so that conformance tests run cleanly across backends with
different capabilities, and so backend-independent invariants live in one place.

## Scope

- `storeutil.ValidateID` additionally rejects path separators (`/`, `\`), NUL,
  and ASCII control characters. These were always latent hazards in fsstore
  (NUL crashes file creation on POSIX; `/` silently nests directories) and
  will be correctness requirements for any nested-bucket backend.
- New conformance test `CreateEntityRejectsInvalidIDs` enforces the rules
  uniformly across fsstore and memstore.
- `storetest.RunAll` takes a `Capabilities` struct; attachment tests are gated
  on `Capabilities.Attachments`. The attachment-key slash check moves from
  `validation.go` into `attachment.go` where it belongs.

## Acceptance criteria

- `go test ./internal/store/...` passes for fsstore, memstore, and storetest.
- `CreateEntityRejectsInvalidIDs` runs on both fsstore and memstore.
- No existing conformance tests regress.
- Zero violations of the tightened rules in committed entity IDs in this repo.
