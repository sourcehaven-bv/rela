---
id: RR-CICPB
type: review-response
title: script.NewReaderRuntime/NewWriterRuntime option ordering lets caller clobber loaded secrets/AI
finding: 'internal/script/runtime.go:20-22, 35-37 — ctxOpts (AI, secrets) are appended first, user opts after. Since WithSecrets/WithAIProvider are last-wins setters, a caller that passes WithSecrets(nil) or WithAIProvider(nil) wipes the loaded values. No current caller does this, but the API invites the footgun. Fix: swap order so user opts apply first, ctx opts last — then context state from .rela/ cannot be accidentally overridden.'
severity: significant
resolution: 'Swapped option order in script.NewReaderRuntime and NewWriterRuntime: caller opts applied first, context opts (AI + secrets from .rela/) applied last. Context wins on conflict. Docstring updated to document the precedence. Loaded AI/secrets cannot be clobbered by caller-supplied nil-valued options.'
status: addressed
---
