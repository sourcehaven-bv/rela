---
id: RR-N5ZY6L4
type: review-response
title: string.format size cap bypassed via %[n] / * and raw LValue arguments
finding: Go's %[n] argument indexes and * widths let one large argument be repeated past the 16 MiB output cap. Raw LValue arguments reached Go's fmt directly, so the size estimate missed some strings and a table could print Go internals.
severity: significant
resolution: 'Fixed in 6ed495060: string.format parses the Lua 5.1 format itself and refuses %[n] and *. Fixed further in 3a5b2d9ff: each argument is converted as Lua 5.1 does before fmt sees it (numbers for numeric verbs, tostring for %s, string or number for %q). %x of a non-numeric string is refused. Pinned by bounds_test.go.'
status: addressed
---
