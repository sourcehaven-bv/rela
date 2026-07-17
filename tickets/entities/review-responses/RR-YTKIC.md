---
id: RR-YTKIC
type: review-response
title: Two accepted spellings for not-equal (!= and ~=) may confuse authors and split tests
finding: 'The spec accepts both `!=` and `~=` for not-equal (`~=` for Lua/predicate congruence, `!=` for familiarity). Supporting both is cheap in the parser but doubles the surface authors must recognize and invites inconsistent config across the codebase. Recommend: pick ONE canonical spelling in authored config and docs (lean `!=` since form authors aren''t Lua users), accept the other silently as an alias if desired, but document only the canonical one. Ensure the divergence from predicate (which uses `~=`) is noted where the ''congruent grammar'' claim is made — `!=` is NOT valid Lua, so strict congruence already has an asterisk.'
severity: minor
resolution: Canonical spelling is `!=` (docs show only this); `~=` accepted as a silent alias for predicate congruence. Spec notes the asterisk that != is not valid Lua and the engine is permissive vs predicate strict, so 'congruent' means surface/precedence-aligned, not semantically identical.
status: addressed
---
