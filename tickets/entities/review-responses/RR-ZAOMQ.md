---
id: RR-ZAOMQ
type: review-response
title: io.LimitReader at 10 MiB is the wrong place to cap value size
finding: The plan caps the disk read at 10 MiB and the set-time JSON at 10 MiB. But JSON encodes many Lua tables with ~2-3x overhead, so a 10 MiB JSON file decodes to a few MiB of live Go value. Also `io.LimitReader` at exactly 10 MiB silently truncates rather than errors.
severity: significant
resolution: 'Resolved by scope change: no disk in v1, so there''s no JSON encoding, no file read, no `io.LimitReader` to size. Value representability check is a walk over the Lua value rejecting unsupported types, not a JSON round-trip (see AC 10 and Notes section). Revisit for v2 disk backend.'
status: addressed
---
