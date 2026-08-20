---
id: RR-IVIPQA
type: review-response
title: Pointer grammar permits -- and collides with the relation key separator
finding: 'Design doc §3.2 grammar `<pointer> ::= [a-z][a-z0-9-]*` permits consecutive hyphens. Relation keys and fs filenames are `from--type--to` (fsstore/relation.go:17), and with state tails the FROM slot must serialize the pointer (two edges differing only in FromPointer must be distinct files/keys), so a pointer like `a--b` makes `PAGE-1@a--b--implements--SPEC-9.md` ambiguous to parse. The doc''s separator analysis (§3.2: "-- is taken") checked the base-id, not the pointer slot. Proposed fix: tighten the pointer grammar to `[a-z][a-z0-9]*(-[a-z0-9]+)*` (no leading/trailing/consecutive hyphens) — free, since pointer names are metamodel-declared and none exist yet. Amends §3.2, so it needs the architect.'
severity: significant
resolution: 'Architect amended design doc §3.2 (2026-08-20) to the proposed grammar exactly: `[a-z][a-z0-9]*(-[a-z0-9]+)*`, no leading/trailing/consecutive hyphens, noting the old grammar''s `--` collision with the relation-key separator. Consistent with ValidateID''s existing no-consecutive-hyphens rule (documented as load-bearing for the storage format). Implemented in internal/entity/pointer.go''s pointerPattern with the collision rationale in its godoc.'
status: addressed
---
