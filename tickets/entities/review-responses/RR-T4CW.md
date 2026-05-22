---
id: RR-T4CW
type: review-response
title: Several plan-clarity gaps (AC4 inline example, public API surface, BOM, internal/lua in arch-lint, AC1 corpus size)
finding: "Five small documentation gaps: (a) AC4 names use cases but no end-to-end worked example inline — add one (env + bindings + expected eval) so the implementer doesn't chase references; (b) public-vs-unexported API surface not declared — in particular whether Value is interface (sum type) or struct-with-tag; pick interface for Lua-shaped values; (c) BOM/UTF-8 marker behaviour unspecified — strip a leading \uFEFF before prepending `return ` or reject explicitly; (d) AC8 forbidden-import list doesn't include internal/lua, but the whole point is to NOT pull in lua.LState surface — add it; (e) AC1 accept corpus size unspecified — pin a number (≥15 files covering all six shapes' use cases) so future drift is visible."
severity: minor
resolution: 'All five gaps addressed: (a) worked lifecycle example added in plan + will live in doc.go; (b) Value is a sealed interface with Bool/Number/String/Nil/Record/List variants; (c) preprocessor strips leading UTF-8 BOM (AC11); (d) internal/lua added to forbidden imports in AC8; (e) AC1 pins ≥15 files covering all six shapes.'
status: addressed
---
