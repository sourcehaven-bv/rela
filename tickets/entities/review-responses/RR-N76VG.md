---
id: RR-N76VG
type: review-response
title: Cards/content cards should also become real anchors (out of scope for this fix)
finding: 'While we''re in this code: <article @click> for cards has the same middle-click/cmd-click/screen-reader issues as <a @click> had for list. Wrap each <article> in <a> for full a11y. Out of scope for this bug, but flagged for a follow-up ticket.'
severity: nit
reason: Cards/content-cards have the same middle-click/cmd-click/screen-reader issues as list, but converting <article> to <a> requires CSS restructuring (cards-grid relies on article semantics) and is a separate UX enhancement. Out of scope for this bug fix; will file follow-up ticket.
status: deferred
---
