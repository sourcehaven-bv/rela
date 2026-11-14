---
id: TKT-35BY3PE
type: ticket
title: 'Workshop step mode for pages (mode: steps, present mode)'
kind: enhancement
priority: medium
effort: m
started: "2026-11-13"
completed: "2026-11-13"
status: done
---

## Description

First slice of a workshop mode in rela, motivated by the DPIA session mockup
(`dpia-sessie.html`): a facilitator walks a room through a fixed series of
screens on one projected display. No phones, no voting, no multi-device sync.

An entity page already shows several views of one entity as tabs, each scoped to
that entity. This ticket adds a second way to render the same page: as ordered
steps.

- `mode: steps` on a page renders its tabs as a numbered stepper with Back and
Next, instead of a tab bar. Default `mode: tabs` keeps today's behaviour.
- `prompt:` on a tab of a steps page shows a large headline above the view
(for example "Het is 2028. Deze chatbot staat in de krant.").
- Present mode (`?present=1`) hides the sidebar and the next-action card,
scales up the type, and offers browser full screen. Leaving present mode
restores the normal frame.
- PageDown/PageUp (what presentation clickers send) move between steps when
focus is not in an editable field.

## Out of scope (follow-up tickets)

- A step that shows the entities of a set one at a time.
- Document and dashboard tabs on entity pages.
- `done_when` warnings per step, a timebox per step.
- Kanban quick-add, creating a column from the board, coloured matrix cells.
- Phones, voting, guests, shared step state between screens.
