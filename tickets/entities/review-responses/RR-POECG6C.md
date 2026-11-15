---
id: RR-POECG6C
type: review-response
title: string.format refused valid Lua 5.1 flags and the guide overstated conformance
finding: The format parser refused five Lua 5.1 flags, and the Lua guide claimed fuller Lua 5.1 string.format conformance than the implementation gives.
severity: minor
resolution: 'Fixed in 13cd61e18: the flag check accepts the Lua 5.1 flags and the Lua scripting guide states the actual format rules. Pinned by the new bounds_test.go cases.'
status: addressed
---
