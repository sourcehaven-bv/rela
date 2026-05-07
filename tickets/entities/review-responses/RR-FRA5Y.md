---
id: RR-FRA5Y
type: review-response
title: 'Accessibility: <a> without href is an a11y bug being incidentally fixed'
finding: '<a class="list-link" @click> with no href is not announced as a link by screen readers, isn''t in tab order without explicit tabindex, and can''t be activated with Enter. The plan adds href but doesn''t acknowledge the a11y angle. Add an a11y verification step: keyboard-navigable, focus-visible style, screen-reader-announced as link.'
severity: significant
resolution: 'Plan adds a manual a11y check step: tab to .list-link, see focus ring, hit Enter to navigate. Real <a href> + click.prevent gives screen-reader ''link'' role and keyboard activation for free.'
status: addressed
---
