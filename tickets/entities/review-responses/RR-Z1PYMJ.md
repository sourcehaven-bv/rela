---
id: RR-Z1PYMJ
type: review-response
title: Sticky edit button slides under the mobile back/scope bar
finding: On <=768px EntityDetail renders .scope-nav.mobile-topbar (sticky top 0, z 102) whenever there is a back target or scope. The rail stuck 4px below the bar's top, inside it, so on a phone the only way into the editor was hidden mid-body (AC3).
severity: significant
resolution: 'Fixed. RlInlineEdit reads --rl-inline-edit-sticky-top. New composable useStickyHeight measures the bar while it is position: sticky (0 in flow, re-measured on resize and ResizeObserver); EntityDetail sets the variable to bar height + space-1. Pinned by e2e ''on a phone the edit button stays below the sticky back bar'' and useStickyHeight unit tests; measured before fix: bar 56-104px, button at 60px.'
status: addressed
---
