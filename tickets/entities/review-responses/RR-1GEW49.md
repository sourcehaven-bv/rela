---
id: RR-1GEW49
type: review-response
title: Long upload names overflow the storage key
finding: The storage key adds a 17-byte token prefix to the display name, so a name near the 255-byte limit produced a key the filesystem backend rejects.
severity: significant
resolution: capNameLen caps the display name on a rune boundary, keeping the extension, so token, name and a collision suffix fit 255 bytes. The fsstore temp file name is now a fixed-length digest instead of prefix plus name, which overflowed on Linux. TestService_LongNameIsCapped (multi-byte and ASCII cases).
status: addressed
---
