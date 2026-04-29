---
id: RR-RB0EJ
type: review-response
title: data-entry-ui Key UI components list overstates what exists as named components
finding: Description enumerates 'Page header with title and actions' (no PageHeader.vue — it's a repeated pattern) and 'Jump bars for entity navigation' (only inside CustomView.vue's sectioned-view jump links, not a standalone component). Either tighten the list to actual .vue files in frontend/src/components/, or replace the enumeration with a one-liner pointing at the component tree.
severity: minor
resolution: Replaced the 'Key UI components' enumeration with a one-liner pointing at the actual component tree (frontend/src/components/{ui,forms,lists,entity,common}). More durable than naming components that don't exist as standalone files.
status: addressed
---
