---
id: RR-FARSAFM
type: review-response
title: refsByTypes allowlist entry lacks the pushdown caveat
finding: On the load-then-filter fallback ScriptReader.ListEntities reads the whole type before the first row so stopping early bounds what is returned not what is read.
severity: minor
resolution: The bounded entry now carries the same caveat as luaListEntities.
status: addressed
---
