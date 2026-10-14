---
id: RR-SIHAQL
type: review-response
title: Other code enumerating project areas
finding: internal/docscapture/server.go copies a fixed list of directories; captures of scripts using require would fail without lib.
severity: minor
resolution: Added lib to internal/docscapture/server.go's copied directories; no other code enumerates project areas.
status: addressed
---
