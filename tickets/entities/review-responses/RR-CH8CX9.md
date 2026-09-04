---
id: RR-CH8CX9
type: review-response
title: Budget tests should pin size-independence, not guessed constants
finding: AC2 states numeric budgets (≤ 6, ≤ 8, ≤ 4) that are estimates. The ACL request resolution also issues store calls (member-of closure via the store-backed acl.Graph), so the constant is wiring-dependent. A fixed number will either be wrong on day one or be tuned to whatever the implementation happens to do.
severity: minor
resolution: 'AC2 rewritten: budget tests assert that the store-call count for a 10-row page equals the count for a 50-row page (size independence) and pin the measured constant afterwards with a comment recording the pre-change count; the numbers in the plan are targets, not assertions.'
status: addressed
---
