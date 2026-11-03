---
id: TKT-7VLJ2SV
type: ticket
title: 'Property test: kanban moves and list panel with live refresh'
kind: test
priority: medium
effort: l
status: backlog
---

## Description

Extend the fake with list filters and status changes, with matching
contract-test generators. Property: a card ends in the column matching its
server status; no move is lost or applied twice under live refresh.

Part of the SPA property-testing plan. Start after TKT-7LTLHW3 merges; it
provides the fake API, the contract test and the scheduler setup.
