---
id: RR-R0KU49G
type: review-response
title: Run card ends under the sticky top bar on mobile
finding: On a phone the entity page's sticky top bar covers the card's title after it is scrolled to the top of the pane.
severity: significant
resolution: 'The run card has scroll-margin-top: var(--rl-inline-edit-sticky-top); EntityDetail sets that variable to the bar''s height.'
status: addressed
---
