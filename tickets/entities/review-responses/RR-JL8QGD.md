---
id: RR-JL8QGD
type: review-response
title: Mixed binary versions refuse the whole file
finding: 'realtimecfg decodes with KnownFields and keyChecker reports unknown keys. An older rela, rela-desktop or stdio MCP binary that loads a realtime.yaml with in: refuses the file and runs realtime degraded on the last good snapshot, or off. This fails closed, but an operator who adds in: while a CLI or desktop client on an older release shares the project will see realtime stop there. Add one line to the docs (and the release note) saying in: needs every process on this release or later.'
severity: minor
resolution: Q4, the docs and a release-note item now say that every process loading the project (rela-server, stdio `rela mcp`, `rela channel`, rela-desktop) needs this release or later, and the risk is listed.
status: addressed
---
