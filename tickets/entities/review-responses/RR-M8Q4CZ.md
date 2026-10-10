---
id: RR-M8Q4CZ
type: review-response
title: Migration title cut by UTF-16 units
finding: The client cut at 120 UTF-16 units while the server limits 120 bytes.
severity: minor
resolution: Cut to 120 UTF-8 bytes without splitting a character.
status: addressed
---
