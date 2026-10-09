---
id: RR-IH9UDU
type: review-response
title: Focus lost when a column collapses or expands
finding: The collapse button and the rail are swapped by v-if, so the focused control unmounts and focus falls to body on every toggle.
severity: significant
resolution: 'Added useSectionToggleFocus in rela-components: RlBoard and RlSwimlaneBoard mark the toggled column and, after the caller re-renders, focus its counterpart control (rail after collapse, collapse button after expand). Pinned by the Collapsible / CollapsibleColumns story play tests, which fail with the focus call removed.'
status: addressed
---
