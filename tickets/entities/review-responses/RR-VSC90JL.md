---
id: RR-VSC90JL
type: review-response
title: "nav status primes even when no condition will redact"
finding: "nav_status.go: with no list condition and no rule condition nothing redacts, yet the whole population is primed. listPage avoids this."
severity: minor
resolution: "navStatusRows now primes only when the list has a condition, as listPage does. navStatusFor primes when a rule has a condition and the list has none. This also stops a second prime for dashboard cards, which redactRows primes already. Added a rule-only budget case (7 reads; 503 at 50 rows without the fix)."
status: addressed
---
