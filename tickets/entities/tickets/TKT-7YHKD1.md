---
id: TKT-7YHKD1
type: ticket
title: Create forms need a "Create & add another" button for repeated entry
kind: enhancement
priority: medium
effort: l
status: done
---

## Problem

The data-entry create form has one terminal save action: on success it navigates
away (to the newly created entity or back to the list). Bulk-ish entry — adding
five tickets, ten contacts, a batch of notes — means repeating list → "+ New" →
fill → save → navigate for every single record. The navigation is pure overhead
for the "I have a stack of things to enter" workflow.

## Proposal

Add a secondary action on the create form (edit forms are unaffected) that:

1. Submits the form exactly as the primary save does.
2. On success, stays on the create route and resets the form to a fresh
create state rather than navigating to the created entity.
3. Confirms the create happened (the created entity should be visible/reachable
somehow — toast with a link, at minimum).

## Resolved in planning (PLAN-ONV2PB)

- **Sticky fields:** not inferred. A new per-field `data-entry.yaml` key,
`keep_on_add_another: true`, marks the fields and relations that carry over; the
reset is clean otherwise. Inferring stickiness from URL pre-fills was considered
and rejected — it makes the behavior depend on how the user happened to reach
the form, and gives the operator no way to express intent.
- **Wizard forms:** yes. The action renders beside Create on the last step
(both share the `wizard.isLastStep` gate) and returns the form to step 1.
Embedded/inline-create forms are excluded — their host expects exactly one
entity to link.
- **Confirmation:** a success toast naming the created entity's id. The `Toast`
type has no action/link field, so a clickable toast would mean changing the
toast store and every toast in the app; that is a follow-up, not a prerequisite.
