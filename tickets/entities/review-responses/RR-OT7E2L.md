---
id: RR-OT7E2L
type: review-response
title: composeReadScope ok=false must not mean allow-all
finding: ok=false mixes no-policy with cannot-compose; the roll-up has no load-then-filter fallback.
severity: significant
resolution: 'visibility.Rollups branches explicitly: no gate wired means ungated; deny means no rows; a scope that cannot be composed omits the value.'
status: addressed
---
