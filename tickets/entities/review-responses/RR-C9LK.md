---
id: RR-C9LK
type: review-response
title: Use script return value via MultRet instead of rela.respond
finding: RunFile already uses PCall MultRet - return values are on the stack, just discarded. Adding RunActionFile that pops Top(-1) is fewer lines than respond+state. More Lua-idiomatic too.
severity: significant
resolution: Scripts use idiomatic Lua return statement. New RunActionFile method reads L.Get(-1) after PCall. No rela.respond needed.
status: addressed
---
