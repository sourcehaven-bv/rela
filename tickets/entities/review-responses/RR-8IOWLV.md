---
id: RR-8IOWLV
type: review-response
title: Retry tests do not exercise the paths they name
finding: Retry tests do not exercise the paths they name
severity: significant
resolution: Tests use one fake world shared by reader, writer and re-read. They assert a create loser re-finds the winner, a lost CAS re-reads the base, the exact attempt count and the wrapped error, and cancellation between attempts.
status: addressed
---
