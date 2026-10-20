---
id: RR-NTGOJU
type: review-response
title: The `satisfies` golden does not catch most drift
finding: '§0 "Drift is pinned by a golden", AC 2. A JSON module import is not an object literal, so `satisfies` does no excess-property check: a field Go adds, or renames while the TS side has it optional, passes. JSON imports also widen literals, so a union such as `state: ''open''|''closed''|''archived''` fails on a valid file. The library tsconfig has no `resolveJsonModule` (only frontend/tsconfig.json does). Fix: generate the TS types from the OpenAPI document the server already serves (openapi-typescript, checked in, with a CI diff), or have the Go golden test also emit the field set per type and a vitest compare it to a hand-kept key list. Keep the golden only as test and story data.'
severity: significant
resolution: The Go golden test also writes the field set per wire type, and a vitest compares it with a key list kept next to the TS types (R11, AC 20).
status: addressed
---
