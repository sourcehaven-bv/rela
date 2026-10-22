---
id: RR-FCZTWT
type: review-response
title: RELA_AFFORDANCE_PROFILE none/demo bypasses client field ceilings on REST
finding: ResolverFromProfile applies the profile override before the policy, so none/demo drop field ceilings on REST while MCP/CLI still redact.
severity: significant
reason: Pre-existing and a documented operator override (DR-M3), not introduced by this fix. Changing the profile precedence is a design decision for the maintainer, so it is tracked separately as BUG-UO72G2 with options.
status: deferred
---
