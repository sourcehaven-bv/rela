---
id: RR-T3O79I
type: review-response
title: Tools added by a later reload need a restart
finding: 'registerCommentTools runs once, so a comments: block added during a session stays unregistered'
severity: minor
resolution: Documented in docs/mcp-server.md. The drop direction is pinned by TestCommentTools_AfterAReloadDropsComments.
status: addressed
---
