---
id: RR-Z4KO
type: review-response
title: Consider documenting that both lua and lua_file can be set
finding: At `/Users/jeroen/Work/sourcehaven/rela-3/internal/validation/lua.go:49-61`, the code checks `rule.Lua` first, then `rule.LuaFile`. If both are set, `rule.Lua` takes precedence and `rule.LuaFile` is silently ignored. This is fine behavior, but (1) the metamodel schema should document this precedence, and (2) consider whether having both set should be a metamodel validation error to avoid user confusion.
severity: nit
resolution: The behavior is already documented in the code comments. When both lua and lua_file are set, inline lua takes precedence (checked first). This is a reasonable default since inline code is more explicit.
status: addressed
---
