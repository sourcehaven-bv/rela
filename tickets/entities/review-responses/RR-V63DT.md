---
id: RR-V63DT
type: review-response
title: PROJECT_ROOT assumes fixture location
finding: path.resolve(__dirname, '../..') breaks silently if fixtures.ts moves. Walk up to go.mod or centralize the constant.
severity: nit
reason: Nit. PROJECT_ROOT = path.resolve(__dirname, '../..') is a common pattern; walking up to find go.mod adds complexity for negligible benefit. Revisit if fixtures.ts moves.
status: deferred
---
