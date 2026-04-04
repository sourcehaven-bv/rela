---
id: RR-D50C
type: review-response
title: rela.output() and rela.write_file() in validation context
finding: 'The Lua runtime registers `rela.output()` and `rela.write_file()` bindings. In validation context: output() would write to stdout which may be unexpected/noisy; write_file() would fail silently (no output dir configured) or should be explicitly blocked. Consider: (1) not registering these for validation runtime, or (2) documenting that they''re no-ops/blocked in validation context.'
severity: minor
resolution: Use io.Discard as stdout for validation runtime, write_file fails gracefully with no output dir
status: addressed
---
