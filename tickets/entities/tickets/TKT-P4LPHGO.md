---
id: TKT-P4LPHGO
type: ticket
title: Flow run panel for people who only do a step
kind: enhancement
priority: high
effort: m
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

## Description

Make the flow run panel on an entity page fit for a person who must do a step
but does not manage the flow, such as a customer completing a self-service
signup.

- The step form shows one labelled, full-width field per row. It used the
12-column property grid without grid items, so every field took one column and
the labels overlapped.
- Fields, states, flows, branches and timers without a label show their
name in words (`check_vat` reads as "Check vat").
- A flow may set `manage: { permission: P }`. P then cancels and retries a
run instead of the start permission. `_actions.manage` reports it, and the card
shows roles, guards, timers, job attempts, the diagram and the timeline only
when it is true. Others see one plain line.
- With `per_subject: one`, Start is hidden while a run of the flow on the
entity is unfinished. `/_flows` returns `per_subject`.

## Acceptance

- A customer who starts the signup and does its step sees a vertical
form and no guard, roles, diagram, timeline or cancel button.
- Staff still see all of it.
- Unit, appbuild and e2e tests cover each point.
