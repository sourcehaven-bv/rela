---
id: RR-E3LY
type: review-response
title: syncingFromUrl gate is fragile under realistic conditions
finding: If loadEntities throws between gate-set and gate-clear, gate stays stuck and watcher is permanently disabled. Two filter changes in same tick cause one extra reload. Vue async watch + nextTick interleaving makes timing non-obvious. Use lastWrittenQueryString comparison instead — self-healing, no try/finally dance.
severity: significant
resolution: Replaced syncingFromUrl gate with lastWrittenSig string comparison. Self-healing under errors, no try/finally dance, no stuck state.
status: addressed
---
