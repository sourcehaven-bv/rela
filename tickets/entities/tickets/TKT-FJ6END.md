---
id: TKT-FJ6END
type: ticket
title: Lua scripts cannot distinguish an ACL-redacted property from a genuinely-unset one
kind: enhancement
priority: medium
effort: m
status: done
---

Entities reaching Lua via `visibility.ScriptReader` are field-redacted: hidden
property names are stripped from `Properties` entirely. A script therefore sees
`nil` for both "you may not read this" and "nobody ever set it". Scripts that
render reports need to show `[redacted]` rather than a blank, so the redaction
fact must be observable from Lua.

## Problem

`visibility.Redact` (`internal/visibility/policyreader.go:184`) strips hidden
property names from `entity.Properties` outright. Every read-out path goes
through it — `ScriptReader.GetEntity`, `ListEntities`, the pushdown `RedactRow`
seam, and (see below) three data-entry HTTP paths.

`EntityToTable` (`internal/lua/runtime.go:1300`) then copies `e.Properties` into
the Lua `properties` sub-table. A stripped property and an absent property are
indistinguishable: both are `nil`, and `entity:prop(name, default)` returns the
default for both.

## Why this is not a confidentiality regression

Per the root CLAUDE.md rule, field-level `visible:` redaction hides property
**values only** — it makes no claim to conceal *which* properties exist, since
the metamodel is served over `/api/v1/_schema`. A "field-existence oracle" is
not a threat this guards against.

Direct precedent on the HTTP surface: DEC-T0XIWQ already ships the `_redacted`
wire field (`redactedPropertyNames`, `internal/dataentry/affordances.go:846`),
listing redacted property names to the SPA for exactly this reason. This brings
the Lua surface to parity.

Row-level gating is untouched: a hidden *entity* stays nonexistent to Lua
(`ScriptReader.GetEntity` returns `store.ErrNotFound`).

## Solution

A new `entity.Redacted []string` field, populated in `visibility.Redact` and
exposed to Lua as `entity.redacted` (a set) plus `entity:is_redacted(name)`.

**`entity.Inaccessible` was considered as the carrier and REJECTED**, despite
its godoc explicitly inviting "Lua-driven access control". `IsLocked()` is
`len(Inaccessible) > 0` and gates six sites; two would break:

- `internal/validator/validator.go:198` silently `continue`s past locked
entities, and the validator reads through the *gated* `lateGatedReader`
(`internal/dataentry/app.go:742`). Every entity with an ACL-hidden property
would be silently dropped from validation.
- `internal/dataentry/write_handler.go:335` returns 422
`encrypted_inaccessible` ("run `git-crypt unlock` first") — a nonsense message
and a spurious write block for an ACL redaction.

The concepts genuinely differ: `Inaccessible` means *nobody here can read these
bytes, do not write through it*; ACL redaction means *this principal may not see
this value, but the data is intact and writable*. The misleading godoc was
corrected as part of this ticket.

## Scope

IN scope:

- `entity.Redacted []string` + `IsRedacted(name)`; `Clone` deep-copies it.
- `visibility.Redact` records withheld names, sorted.
- `EntityToTable` exposes `redacted` + `is_redacted()`.
- Docs (via `docs-project/`, the generated-docs source).

OUT of scope:

- Body/content redaction — not policy-expressible; the existing
`TODO(body-redaction)` stands.
- Relation field redaction — relations have no field-level redaction
(RR-BZNL0S).
- Changing *what* is stripped. Values stay hidden; only the *fact* of
redaction becomes observable.
- Wiring an affordance resolver into appbuild to close RR-7408F5 — see
RR-IHWEB0; that changes ACL enforcement scope and needs its own ticket.

## Runtime coverage (important for users)

| Runtime | Row gate | Field redaction | `is_redacted` |
|---|---|---|---|
| Data-entry (documents, views, actions) | yes | yes | meaningful |
| Scheduler (`run_as:`) | yes | **no** | always `false` |
| CLI, MCP, docs | no | no | always `false` |

The scheduler gap is pre-existing (RR-7408F5): `ScheduledLuaWriteDeps` passes a
nil redactor. Documented, not fixed here — see RR-IHWEB0.
