---
id: TKT-QXHFJZ
type: ticket
title: 'Triage modernize omitzero findings: omitempty on non-omittable fields'
kind: chore
priority: low
effort: xs
status: ready
---

## Description

The `modernize` linter (enabled in the modernize-autofix PR) has an `omitzero`
analyzer that flags `,omitempty` JSON tags on fields where `omitempty` has no
effect — most commonly struct-typed fields like `time.Time`, which `omitempty`
never omits (a zero `time.Time` still marshals to `"0001-01-01T00:00:00Z"`).

`omitzero` was **disabled** in `.golangci.yml` when modernize was enabled,
because — unlike the other modernize analyzers — its fix is behavior-adjacent
(it changes wire output), not a pure mechanical rewrite. This ticket triages and
applies the three findings.

## Findings

| Site | Field | Correct fix |
|------|-------|-------------|
| `internal/entity/entity.go:48` | `Entity.UpdatedAt time.Time` `json:"updated_at,omitempty"` | switch to `,omitzero` (Go 1.24+) |
| `internal/entity/entity.go:214` | `Relation.UpdatedAt time.Time` `json:"updated_at,omitempty"` | switch to `,omitzero` |
| `internal/dataentry/api_v1.go:662` | request DTO `Relations v1.RelationsField` `json:"relations,omitempty"` | drop `,omitempty` (decode-only struct) |

## Analysis

**The two `time.Time` fields** — `omitempty` silently does nothing on a struct,
so a zero `UpdatedAt` currently serializes as `"0001-01-01T00:00:00Z"` in the
CLI `-o json` output and MCP export paths (`internal/output/output.go`,
`internal/mcp/convert.go` — the API/wire layer uses its own `apiwire/v1.Entity`,
not these tags). Switching to `,omitzero` makes it actually omit the key when
unset, which is clearly the original intent. This **is** a wire-output change
(the `0001-...` string disappears when the timestamp is zero), but:

- no golden/snapshot test asserts `updated_at` in JSON output (grep-verified),
- the emitted zero-value string was never meaningful,

so it is a safe, intent-preserving fix.

**The `api_v1.go` request struct** is decode-only
(`json.NewDecoder(...).Decode`); `omitempty` only affects *encoding*, so the tag
is inert here regardless. The field has a custom `UnmarshalJSON` + `IsEmpty()`.
The right fix is to just drop the pointless `,omitempty` rather than add
`,omitzero` to a struct that is never marshaled.

## Approach

1. `entity.go` ×2: `,omitempty` → `,omitzero` on the `UpdatedAt time.Time` fields.
2. `api_v1.go`: drop `,omitempty` from the decode-only `Relations` field.
3. Remove the `omitzero` entry from the `modernize.disable` list in `.golangci.yml`.
4. `golangci-lint run ./...` → 0 issues; `go test ./...`.

## Context

Follow-up to the modernize-linter enablement PR (sibling to TKT-AHUNF "Enable
additional golangci-lint v2 linters"). Deferred from that PR precisely because
these three are the non-mechanical residue of the modernize suite.
