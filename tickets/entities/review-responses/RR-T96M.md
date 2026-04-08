---
id: RR-T96M
type: review-response
title: 'F2: Validation 5s timeout silently caps AI calls'
finding: validationTimeout = 5 seconds, enforced via ls.SetContext(ctx). chatContext(r) reads r.L.Context(), so AI requests from validation rules inherit the validation's 5-second deadline. The OpenAI-compat provider has a 30-second default client timeout that becomes dead code inside validation. Worse, a rule that legitimately needs AI will appear to 'usually work in dev, randomly fail in CI' as soon as provider latency drifts up. The failure mode is an opaque ErrTimeout with no hint that a validation deadline, not the network, caused it.
severity: significant
resolution: 'Resolved by un-wiring AI from validation entirely (see F1). The 5s validation timeout no longer interacts with AI calls because AI is unreachable from validation rules. Tracked as a follow-up: AI in validations must come with longer per-rule budgets and explicit opt-in.'
status: addressed
---
