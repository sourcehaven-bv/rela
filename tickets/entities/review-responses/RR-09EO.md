---
id: RR-09EO
type: review-response
title: assignManagedOrder skip-condition lets malformed values through
finding: |-
    assignManagedOrder at workspace.go:1424-1458 uses `_, present := rel.Properties[outProp]` to decide whether to auto-assign. ANY value counts as present — including "abc", nil, false, time.Time. So an MCP/Lua/CLI caller can write garbage straight through to disk because the wire-format validator only runs on the HTTP path. Three-way policy split (wire rejects, disk tolerates, writers can write garbage) is incoherent.

    Also (Architect C2): assignManagedOrder returns nil silently when relType is not in the metamodel (workspace.go:1425-1428). The caller already validated; this code is unreachable except via metamodel reload race. Either way, silent no-op violates CLAUDE.md "Never substitute a no-op or sentinel implementation silently".

    Fixes:
    1. Only skip when existing value is a finite numeric.
    2. Return error on unknown relation type, not nil.
severity: critical
resolution: 'assignManagedOrder now (1) returns an explicit error when the relation type isn''t in the metamodel (was: silent nil), and (2) uses entitymanager.FiniteOrder to detect a usable caller-supplied value rather than just checking presence. Garbage values ("abc", nil, true, NaN, Inf) are overwritten with AppendOrder ordinals. Added TestCreateRelation_GarbageOrderValueIsOverwritten covering all five cases.'
status: addressed
---
