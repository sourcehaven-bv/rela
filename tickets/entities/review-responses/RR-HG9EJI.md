---
id: RR-HG9EJI
type: review-response
title: Node-view buttons do not react to offer changes
finding: update() only runs on node or decoration changes, so offers arriving after mount would not show buttons.
severity: minor
resolution: 'Plan: offers live in plugin state and the button is a widget decoration rebuilt on offer change; test offers arriving after mount.'
status: addressed
---
