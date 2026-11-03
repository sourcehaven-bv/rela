---
id: TKT-ZTGLU4
type: ticket
title: Rename internal/metamodel to internal/schema
kind: refactor
priority: low
effort: m
status: backlog
description: Mechanical package and type rename to match schema.yaml; move the existing internal/schema helpers first.
---

## Description

The project file has been `schema.yaml` since FEAT-33OKCC, but the code still
says metamodel: package `internal/metamodel`, type `metamodel.Metamodel`. Rename
the package to `internal/schema` and the type to `schema.Schema`.

Origin: archplan thread th-0e0ed196.

## Scope

- Move the existing `internal/schema` (analysis and cleanup helpers, 10
importers) first, for example to `internal/schemaanalysis`.
- Rename `internal/metamodel` to `internal/schema` (565 importing files, 229
non-test); fix the 42 local identifiers named `schema` that would shadow it.
- Update arch-lint config, docs, CLAUDE.md files and rules that name the
package.
- One mechanical PR, landed at a quiet moment after TKT-WFS778 and the
dataentry split merge, to limit conflicts with open branches.
- Depends on the Metamodel split ticket landing first.

Out of scope: user-facing docs that already say "schema"; historical tickets.
