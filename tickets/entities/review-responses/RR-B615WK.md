---
id: RR-B615WK
type: review-response
title: Alias justification cited the consumer-side interface rule backwards
finding: The doc on versionSweeper justified keeping a local name by saying 'CLAUDE.md asks consumers to name the minimum interface they use at the call site'. But these are type ALIASES of the store-package interfaces, not locally-declared minimal ones — the rule cited is precisely the thing the code is not doing. The justification would mislead the next person into thinking an alias satisfies the consumer-side rule.
severity: minor
resolution: 'Rewrote both doc comments to say what is actually true: at one method there is nothing to narrow, so the alias costs nothing the rule is meant to protect, and the local name exists so the call site and the compile-time assertions refer to one identifier.'
status: addressed
---
