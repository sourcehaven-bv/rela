---
id: RR-A6W4T1
type: review-response
title: Masking too coarse for slow-route reports
finding: View, document, gantt, feed and property names were masked, so the log could not say which view was slow; they are config, not secret.
severity: significant
resolution: buildRouteWords adds property names and the data-entry config names (forms, lists, views, kanbans, calendars, gantts, documents, feeds, commands, actions, webhooks, pages, next_actions).
status: addressed
---
