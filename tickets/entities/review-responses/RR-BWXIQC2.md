---
id: RR-BWXIQC2
type: review-response
title: Race tests can match an unrelated lock waiter
finding: The sqlite waiter matched any goroutine blocked on the write mutex, and the stack dump could be cut off silently.
severity: minor
resolution: The sqlite waiter now requires the test's caller frame in the stack and grows the buffer until the dump fits. Postgres cannot tell which goroutine a session serves; that is documented on the helper.
status: addressed
---
