---
id: RR-K4MZMV
type: review-response
title: 'Reconsider neoq for the ephemeral tier: a channel-based pool would avoid the whole upstream hazard class'
finding: 'Reviewer''s leverage note: neoq has cost us three data races, a worker loop fatal on normal conditions, silent payload-hash dedup, an unstoppable cron goroutine, and a second connection pool — and drags in golang-migrate, zap, robfig/cron and guregu/null. Since the fs tier is ephemeral by design, a bounded worker pool over a channel would be roughly 150 lines with none of these problems, leaving neoq to carry only the postgres tier where durability earns its keep.'
severity: minor
reason: 'A real point, deferred rather than dismissed. The containments are now in place, tested, and pinned by conformance cases that fail loudly if regressed, so this is not urgent. But the argument is strong: every hazard fixed here was upstream, and the ephemeral tier gets no benefit from the dependency. Worth its own decision entity before the choice ossifies — revisit if a fourth upstream defect appears or if the memory backend needs behaviour neoq resists.'
status: deferred
---
