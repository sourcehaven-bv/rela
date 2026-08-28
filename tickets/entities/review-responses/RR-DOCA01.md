---
id: RR-DOCA01
type: review-response
title: 'refuses{} passes forever for a principal that does not exist'
finding: 'who= was free text, never resolved against acl.yaml assignments. A principal with no assignment has no grants and is refused BY CONSTRUCTION, so every refuses{} with a misspelled who was green permanently and could never fail. The shipped example manual contained refuses{who="carol@example.com"} — a deliberate claim about an unassigned principal, byte-identical to a typo.'
severity: critical
resolution: 'who= is now resolved against policy.Assignments and an unknown one is refused, listing the assigned principals. Because "this principal has no role" is a legitimate claim, unassigned=true states it explicitly; asserting it on an assigned principal is also an error. The example manual now carries unassigned=true. Pinned by a compiling mutant and a negative-control row.'
status: addressed
---
