---
id: RR-UQHT7O
type: review-response
title: Validation runtime keeps module state across entities
finding: One runtime serves every entity for a rule, so a module's top level runs once; a module reading the entity global at load time keeps the first entity for all later ones.
severity: minor
resolution: Documented in docs/lua-scripting.md (do not read per-call globals at module top level; pass the entity). Pinned by TestLuaValidation_RequireAcrossEntities (module loads once; functions see each entity).
status: addressed
---
