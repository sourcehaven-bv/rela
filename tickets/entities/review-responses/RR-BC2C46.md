---
id: RR-BC2C46
type: review-response
title: 'Minor cleanups in writeopts.go: per-call map rebuild, hoisted type assertion, inconsistent error return, Go slice syntax in a Lua-facing message'
finding: 'Four small issues in internal/lua/writeopts.go. (1) rejectUnknownOptKeys rebuilt its allowlist map from the slice on every call, inside bindings that run in script loops — and the slices were package-level vars specifically so the parser and its test share one source of truth. (2) The error interpolated the slice with %v, showing Lua authors Go syntax: ''accepted: [face content]''. (3) readWriteOptKeys asserted lua.LString for every value before the per-key switch, so the next non-string option (a bool, a nested table) would have to either loosen the check for everyone or restructure the loop. (4) parseWriteOpts returned `o` on the default error branch while the table branch returned writeOpts{} — harmless today since o is zero there, but inconsistent five lines apart.'
severity: minor
resolution: 'All four applied: prebuilt createEntityOptSet/createRelationOptSet via an optKeySet helper and threaded them through; strings.Join for the message (''accepted: face, content''); moved the type assertion into each case with a comment explaining why it is not hoisted; returned writeOpts{} consistently. Tests and lint green after.'
status: addressed
---
