---
id: TKT-QETTR
type: ticket
title: Soften workspace write validation per DEC-HWZHA
kind: enhancement
priority: medium
effort: m
status: planning
---

## Problem

`workspace.updateEntity` and `workspace.createEntity` (workspace.go:836, 990)
both reject with hard 422 errors when `metamodel.ValidateEntity` reports any
issue. This includes soft conditions like:

- A required property is missing or empty
- A property's value type doesn't match the declared schema
- An unknown property is set on the entity (closed-schema violation)

Per **DEC-HWZHA — Validation policy for write APIs**, these are exactly the soft
conditions that should surface as **non-blocking warnings (200 + warnings
array)**, not hard 422s. A hand-editor can produce all of these states in a
markdown file; rejecting them on the next API write creates the same hostile
asymmetry DEC-HWZHA was written to prevent: file system tolerates the state, API
doesn't.

This is the validation-policy migration that DEC-HWZHA's "Migration" section
explicitly reserves for separate-ticket follow-up. TKT-6WLSW shipped the
warnings response field on the PATCH endpoint; this ticket actually emits
warnings instead of 422s for write-time validation.

## What stays 422 (structural impossibilities)

Two `ValidateEntity` error classes stay hard 422 because the storage layer
literally can't proceed:

- **`ValidationErrorUnknownType`** — unknown entity type. The file path encodes
the type; we can't construct a path for a type we don't know.
- **`ValidationErrorIDPrefix`** — ID doesn't match any declared prefix for the
type. Same reason — the file path uses the prefix.

## What softens to 200 + warning

`ValidateProperties` errors:

- **Required field missing / empty** → warning code `required_property_unset`
- **Type mismatch** → warning code `property_type_mismatch`
- **Closed-schema violation (unknown property key)** → warning code `unknown_property_key`

Each warning is `{code, path, detail}` per TKT-6WLSW's wire format. The `path`
is `/properties/<name>` (RFC 6901 escaped).

## Caller migrations (one PR)

Every caller of `workspace.updateEntity` / `createEntity` currently expects 4xx
errors on validation failures. They need to consume warnings instead:

| File | Today | After |
|---|---|---|
| `internal/dataentry/api_v1.go` (PATCH handler) | propagates `*newValidationError` to 422 | adds warnings to response, no longer 422 for soft conditions |
| `internal/dataentry/handlers_api.go` (legacy POST/PATCH) | 422 on error | warnings in response |
| `internal/mcp/tools_entity.go` (create/update tool) | error string | warnings appear in tool result text |
| `internal/cli/create.go`, `update.go`, `set.go` | error → exit 1 | warnings printed to stderr, exit 0 |
| `internal/lua/runtime.go` (`rela.create_entity`, `rela.update_entity`) | Lua error | warnings in result table; script can decide |

The MCP tool changes the tool's user-visible behavior (warnings now appear in
the result text rather than as errors). This is a deliberate semantic shift
aligned with DEC-HWZHA — AI clients can see "you wrote this entity but it has
issues" instead of "this entity was rejected."

The Lua semantic shift is similar — scripts that previously caught errors on
validation issues now succeed and have access to warnings via the result.
Document migration: scripts wanting strict validation should call
`rela.validate(...)` (proposed separate API) or check `result.warnings`.

## Out of scope

- Softening relation-write validation (already done in TKT-6WLSW for the
unified PATCH; per-edge endpoints are TKT-B9SXH territory).
- A new `rela.validate(...)` Lua API for explicit validation calls. If
scripts need strict validation post-this-ticket, they call analyze tools.
- Frontend UI changes to surface warnings inline (handled by TKT-E6094
autosave + general toast/error rework as needed).
- Migration of analyze tools — they're appropriate places for hard checks.

## Relation to other work

- **Blocks**: TKT-E6094 (autosave) — required-field-clear UX needs this
softening to be smooth (rather than rough error+revert)
- **Independent**: TKT-B9SXH (relation widget migration) — runs on a parallel track

## Acceptance criteria sketch

(Full ACs in PLAN-_ after planning.)

1. `workspace.updateEntity` returns `(*UpdateResult{Warnings: [...]}, nil)` for
soft conditions instead of `(nil, *ValidationError)`. Backwards-compat shim
retains the old behavior for callers that haven't migrated.
2. `workspace.createEntity` likewise.
3. `entitymanager.CreateResult` and `UpdateResult` gain a `Warnings []Warning`
field (matching TKT-6WLSW's shape).
4. `PATCH /api/v1/{plural}/{id}` returns 200 + warnings on required-property-
unset; 422 only on unknown type / bad ID prefix.
5. POST `/api/v1/{plural}` likewise (create flow).
6. MCP create_entity / update_entity tool result includes a "Warnings:"
section.
7. CLI `rela create` / `rela update` / `rela set` print warnings to stderr,
exit 0 on soft conditions.
8. Lua `rela.create_entity` / `rela.update_entity` return a result with a
`warnings` table; no script error on soft conditions.
9. All existing integration tests still pass; tests that assert 422 on
required-field-missing get updated to assert 200 + warning.

## Effort

m. Backend validator change is small; the rippling caller updates plus tests add
up. Probably 1–2 days.
