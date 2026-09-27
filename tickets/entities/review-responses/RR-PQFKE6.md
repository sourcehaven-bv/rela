---
id: RR-PQFKE6
type: review-response
title: Scopes compile per request, not per (cfg, meta)
finding: viewQueryScope calls fn(cfg, meta) per call; the plan's hot-reload wording is inaccurate.
severity: nit
reason: Wording only. The plan text is superseded by the implementation; the code comments on QueryScopes describe the per-call compile accurately.
status: wont-fix
---
