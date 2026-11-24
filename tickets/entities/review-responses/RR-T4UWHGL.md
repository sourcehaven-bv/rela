---
id: RR-T4UWHGL
type: review-response
title: Table cards with a limit and no sort may read one page only
finding: A table card that sets a limit below the page size still goes through searchAllEntities; the cut is applied client-side.
severity: minor
reason: Behaviour is correct; it only reads more than needed. Passing the card limit through is a follow-up.
status: deferred
---
