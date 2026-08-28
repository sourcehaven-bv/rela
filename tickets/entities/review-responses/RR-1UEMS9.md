---
id: RR-1UEMS9
type: review-response
title: SMTP test fake leaked a goroutine per abandoned connection, making the package intermittently fail goleak
finding: 'The fake SMTP server registered t.Cleanup closing only the listener. Accepted connections were left open, so when a client dropped a connection without sending QUIT — which go-mail does on several error paths, and which the conformance suite deliberately triggers via rejected messages and cancelled contexts — the per-connection handle() goroutine stayed parked in a blocking bufio Read forever. goleak.VerifyTestMain then failed the entire package. Reproduced roughly 1 run in 3, and only when internal/mail ran alongside other packages (scheduling-dependent), which is the worst shape of CI failure: it would have looked like an unrelated flake and been retried rather than investigated.'
severity: significant
resolution: The fake now tracks accepted connections and its handler goroutines. Cleanup closes the listener, drops every live connection (unblocking the reads), and waits on a WaitGroup so no handler outlives the test rather than racing goleak's snapshot. Verified by rerunning the exact failing condition (the three packages together, cache cleared) five times. Note this was test infrastructure, not production code — the outbox worker's own lifecycle was already clean.
status: addressed
---
