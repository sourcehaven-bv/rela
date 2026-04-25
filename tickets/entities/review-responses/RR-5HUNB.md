---
id: RR-5HUNB
type: review-response
title: Idempotency test typo makes half the assertion dead
finding: document_test.go:417 'if strings.Contains(second, "%2FA") && !strings.Contains(second, "%2FdocB")' — %2FdocB never appears in correct output (/doc/B encodes as %2Fdoc%2FB), so the right-hand clause is unconditionally true. The AND is dead weight. Test also doesn't verify the /B pass is itself idempotent (no third-pass byte equality). A bug where stripQueryKey leaves a trailing & that the second pass doesn't clean would slip past.
severity: significant
resolution: 'Fixed typo: the token is now %2Fdoc%2FA / %2Fdoc%2FB (correctly URL-encoded). Removed the dead-AND. Added a third-pass byte-equality check that runs rewriter(second, /doc/B) and asserts it equals the second pass.'
status: addressed
---
