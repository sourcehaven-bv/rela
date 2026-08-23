---
id: RR-JYXHRT
type: review-response
title: retryOf fell back to RetryBounded, converting a corrupt payload into duplicate side effects
finding: An absent or unrecognized __rela_retry key fell back to RetryBounded, justified in a comment as the 'forgiving' choice. It is the unsafe one. The only way the key goes missing is a corrupt or foreign job — on postgres, a job enqueued by an older build round-trips through JSONB and lands here — and guessing 'retry it' re-runs work whose declaration may have been RetryNever precisely because the side effect must not duplicate.
severity: significant
resolution: 'The fallback is now RetryNever, with an out-of-range value also clamped to RetryNever via Retry.valid(). The comment states the asymmetry explicitly: a missed retry is recoverable, a duplicated payment or mail is not.'
status: addressed
---
