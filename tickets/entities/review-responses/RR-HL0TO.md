---
id: RR-HL0TO
type: review-response
title: AC list split across ticket body (1–9) and plan (10–12); merge to a single 1..N
finding: Ticket body has ACs 1–9, plan adds 'implementation-side' 10–12. The split is awkward for tracking. Renumber to a single 1..12 in the plan and reference TKT-9WZIP for the high-level ones.
severity: nit
resolution: ACs renumbered to a single 1..20 list in the plan, with the test-plan table mapped 1:1.
status: addressed
---

# Finding

The ticket body has ACs 1–9. The plan tacks on three more (10, 11, 12) under
"implementation-side criteria". Cosmetic but annoying: test-plan tables and
review checklists need to map to the same numbering, and split numbering invites
off-by-one mistakes.

# Resolution

Renumber the plan's combined AC list as 1..12. Ticket body stays high-level
(1..9 or even shorter) but plan is the authoritative test-plan map. Re-emit the
test scenarios table against the new numbering.
