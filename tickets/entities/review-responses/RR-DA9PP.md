---
id: RR-DA9PP
type: review-response
title: defaultRegistry constructed at module load
finding: Eight register() calls fire on import before any test installs a console.warn spy. None warn today (no dupes, no type mismatches at registration). Brittle if a future plugin extends defaultRegistry at load.
severity: nit
reason: Nit. The reviewer explicitly noted 'no real bug today'. The eight register() calls at module load all succeed silently (no duplicate names, no supported-type mismatches at registration time). Lazy-init is belt-and-braces work that would land if/when plugin-style widget registration ships. Today there is no use site for it.
status: deferred
---
