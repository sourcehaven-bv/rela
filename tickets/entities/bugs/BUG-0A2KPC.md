---
id: BUG-0A2KPC
type: bug
title: Client baseline drops a role's face grants when the role also holds "*"
description: When a client baseline with an allowlist caps a principal, filterTypes replaces a role's verb list with the baseline list whenever the role list contains "*". Explicit face grants such as beleid@concept are discarded. A bare type grant does not cover a face on writes, so a capped principal (e.g. an MCP agent) loses create/update/delete on that face although both the role and the baseline permit it.
priority: high
effort: s
why1: filterTypes returns the baseline allowlist verbatim when the role list holds "*", so the role's face grants (beleid@concept) are dropped. A bare type grant covers only the default face on a write, so the face is no longer granted.
why2: The wildcard branch reasoned that "*" ∩ allowlist = allowlist and returned early, treating the role list as if "*" were its only entry. On writes "*" covers types' default faces only, so face entries are not subsumed by it.
why3: The face-aware fix in the same function (state grants matched on their type half) was applied to the loop only; the early return above it was left type-granular. Tests covered "*" alone or explicit lists alone, never "*" mixed with face grants.
why4: Write-face semantics ("*" excludes faces) were added after the ceiling compiler, which was designed when a list entry was always a type. Nothing re-checked that every branch of the compiler preserves entries "*" does not subsume.
why5: 'The clamp is tested by example, not by a property: no test asserts that clamping is the exact intersection (role grant kept iff the ceiling permits its type) across mixed wildcard, type and face entries.'
prevention: TestClamp_IsExactIntersection checks the clamp against the exact intersection for every small mix of wildcard, type and face entries under allow and deny ceilings, so a branch of filterTypes that drops or adds an entry fails the build.
started: "2026-10-18"
completed: "2026-10-18"
status: done
---

## Reproduction

acl.yaml with a role holding `create: ["*", "beleid@concept"]` and a client
baseline for `principal_type: agent` with `create: [beleid, ...]`. A request
from an agent principal on `beleid@concept` is denied for create. The same
principal as an interactive user is allowed. Atlas
`verify/mcp-baseline/verify.sh` fails with "agent lost beleid create".

## Root cause

`filterTypes` in `internal/acl/ceilingcompile.go` returns the baseline's
allowlist verbatim when the role list contains `"*"`. Face grants in the same
role list are dropped. A bare type grant covers only the default face on writes
(`GrantsVerbOnState`), so the clamped role no longer grants the face.
