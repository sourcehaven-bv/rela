---
id: RR-FMWYL4
type: review-response
title: Entities-only group flashes a bare heading on first load
finding: groupIsEmpty counted a pending entry as non-empty so a group whose scope matches nothing showed its title for one round-trip before hiding.
severity: minor
resolution: 'Inverted the state: an entry counts as empty until it emits shown(true) for rows or a failure. Test for a pending first fetch added to Sidebar.entities.test.ts.'
status: addressed
---
