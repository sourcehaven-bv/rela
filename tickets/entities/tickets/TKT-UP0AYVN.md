---
id: TKT-UP0AYVN
type: ticket
title: Hide Add to pile on lists with list_buttons
kind: enhancement
priority: medium
effort: s
started: "2026-11-19"
completed: "2026-11-19"
status: done
description: Customer lists show selection checkboxes only because Add to pile is offered. A list_buttons block lets an operator hide Add to pile on lists and search results; with no bulk action left the checkboxes go too.
---

## Description

In a customer portal, lists show bulk-selection checkboxes although the customer
has no bulk action and may not delete. The only thing a selection offers is Add
to pile, which `header_buttons.add_to_pile` does not cover on lists.

## Acceptance criteria

- `list_buttons: {add_to_pile: false | {permission: <name>}}` hides Add to
pile on lists and search results for the principal.
- A list with no bulk action, no deletable row and no Add to pile shows no
checkboxes.
- The block is validated (unknown keys rejected) and documented.
