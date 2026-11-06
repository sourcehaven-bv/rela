---
id: TKT-5CESLHD
type: ticket
title: 'Validation rules enforced on write (enforce: write)'
kind: enhancement
priority: high
effort: m
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

## Description

A custom `validations:` rule never runs on a write: it runs only in `rela
validate` and on the analyze page. A rule such as "when
`versiestatus=ter_beoordeling` then `rechtsgrond` is set" therefore cannot
refuse the transition (OpenVWR R10).

Add `enforce: write` to a validation rule. The write path evaluates an enforced
rule against the new state of the entity, together with the write, and refuses a
violation with 422 (`validation_failed`, error type `rule`).

## Scope

- Allowed on an enforced rule: `entity_type`, `faces`, `when`, `then`, `when_condition`, `then_condition`, `severity` (must be `error` or unset; unset becomes `error`).
- Refused at load: `lua`, `lua_file`, `lua_args` (no Lua on the write path), `relations` and `related(...)` (graph reads), and `content`.
- Conditions compile through `internal/predicate` when the manager is built; a compile error fails the build.
- Paths: create, update, patch, restore, automation writes, copy and conversion targets.
- The rule still runs in `rela validate` and analyze as before.

## Acceptance

- Table-driven tests in the new package and in entitymanager.
- Docs in docs-project (metamodel guide).

## Notes

- `validation:` values are matched exactly; the environment variable is trimmed by its reader.
