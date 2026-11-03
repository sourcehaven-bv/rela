---
id: RR-GNT57F
type: review-response
title: Render without a bound world uses the trivial world
finding: worldFromContext falls back to defaultWorldHandle; faced entities look missing on scheduler paths while the script's own reads use the declared default world.
severity: significant
resolution: 'Plan: bind the declared default world when the context has none (as nextActionSourceWorld does). Test with a faced target type on the scheduler path.'
status: addressed
---
