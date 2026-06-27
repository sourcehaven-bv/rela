---
id: TKT-I5TDVY
type: ticket
title: Extract v1 API wire types into an enforced apiwire package
kind: refactor
priority: medium
effort: m
status: done
---

Give the v1 data-entry API wire vocabulary its own package
(`internal/apiwire/v1`) with an **arch-lint-enforced** boundary, so the
dataentry decomposition contracts become compiler/CI-enforced rather than
same-package convention.

## Why

The dataentry service extractions (visibleReader, affordanceService,
entitySerializer, entityReader) live in `package dataentry`, so their contracts
are convention — any of dataentry's 100+ files can reach their unexported
fields. Reducing App's method count ≠ real boundaries. Giving the shared V1 wire
vocabulary its own package is the keystone: once it exists,
serializer/affordances/handlers can later move to packages that import `apiwire`
(not dataentry), with no cycle.

## Design

- **Top-level `internal/apiwire/v1`** (not nested under dataentry): a future native/desktop bridge will consume the contract in-process (no HTTP), so it's shared between the web serializer and that bridge — not dataentry's privates.
- **Version-as-package** (`v1.Entity`, not `apiwire.V1Entity`): a v2 API is a sibling `apiwire/v2`, k8s-style.
- **Enforced leaf**: arch-lint rule `apiwire mayDependOn {dataentryconfig}` (entity is a commonComponent) and NOT dataentry. Verified a deliberate apiwire→dataentry probe fails arch-lint.

## Slices

- **Slice 1** (merged): the relations-wire cluster (RelationsField/Update/ResourceIdentifier + parser + WireError + JSONPointerEscape) — the one methoded cluster, handled first.
- **Slice 2** (this): the ~48 pure-data response types (Entity, ListResponse, Schema, Config, Sidebar/Conflict/View families, + Mention). apiwire/v1 is now the complete v1 wire contract (54 types).

## Follow-up

Moving serializer/affordances/handlers themselves into apiwire-importing
packages — making those contracts enforced too — is future work (tracked
separately).
