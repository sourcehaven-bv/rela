---
id: RR-WIKBRD
type: review-response
title: 'Concurrent claims: last writer wins, without saying so'
finding: 'Two concurrent `claim` posts both re-enter, and both `set:` writes land, so the last one wins. Guards cannot read the entity, so there is no compare-and-set. Change: state last-writer-wins in the Handoff docs, and point to the notice trail and audit for who held the role.'
severity: nit
resolution: Handoff docs state last-writer-wins for concurrent claims and point to notices and audit.
status: addressed
---
