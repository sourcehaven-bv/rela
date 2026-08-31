---
id: RR-ZECAQE
type: review-response
title: registerAIModule godoc dropped 'registered unconditionally' sentence — which was stale anyway
finding: 'The rewritten registerAIModule godoc no longer says the ai global is ''registered unconditionally'' with a not_configured fallback. The not_configured half moved to the provider field''s Nil: tag (better home); the ''unconditionally'' half was factually wrong even before the refactor — registration is gated on caps.AI, so the global is absent without the capability. The new aiBindings doc states the truth.'
severity: nit
reason: Reviewer confirmed the refactor corrected a stale claim rather than losing a true one; no code or comment change needed. Noted here so the deleted sentence in the diff has an explanation.
status: wont-fix
---

Nit from the TKT-DOPCTI code review (cranky-code-reviewer, PR #1462). Reviewer
also verified: caps.HTTP/caps.AI gates byte-identical; scriptPath closure reads
the live field (SetScriptPath and RunFileContent both observed); the
aiProvider/cache by-value snapshots are safe because their only writers are
Options applied before registerBindings; zero test changes.
