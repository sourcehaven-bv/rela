---
id: BUG-K4NBF2
type: bug
title: Kanban cards and mobile list cards render dangling labels with empty badge pills for unset properties
description: |-
    KanbanView rendered every configured card field row unconditionally: for an unset enum property (effort, tags on most tickets) the row showed a dangling "effort:" label followed by an empty gray Badge pill, because Badge.vue renders a styled chip even for value="". Non-enum fields got a '-' placeholder instead. EntityList's mobile cards had the sibling problem: label rendered with a blank value. On the tickets project's ticket_board nearly every card carried two junk rows ("effort:", "tags:"), which is pure noise — worst on mobile where card space is scarce.
priority: medium
effort: xs
why1: Card field rows rendered for unset values because the v-for over card.fields had no emptiness guard, and the enum branch handed the empty string to Badge, which renders a chip unconditionally.
why2: The field row was designed for the value-present case; the unset case was partially patched (a '-' fallback on the non-enum span) without noticing the enum/Badge branch bypassed that fallback entirely.
why3: No component test rendered a card whose configured field was unset — the tests all seeded complete entities, so the empty-value rendering path was never observed.
why4: 'Test fixtures mirror the happy path: seeded entities always populate every configured property, so "property absent" — the NORMAL state for optional properties across a real dataset — was invisible in tests and only visible when browsing real data.'
why5: 'Systemic: config-driven rendering surfaces (card fields, list columns) multiply the (configured × unset) matrix, and there is no convention that every such surface must define and test its unset-value presentation.'
prevention: 'Unset fields now drop the whole row (visibleCardFields in KanbanView, visibleMobileColumns in EntityList — ACL-locked 🔒 cells intentionally still render). Pinned by the "KanbanView card fields with unset values" tests: unset field renders no row, set field renders normally.'
status: done
---

Fixed on branch `bug/filter-pipeline-and-empty-chips`.

- `frontend/src/views/KanbanView.vue`: visibleCardFields(entity) filters
  card.fields to non-empty values; both board layouts (column + swimlane)
  use it; dead `|| '-'` fallback removed.
- `frontend/src/components/lists/EntityList.vue`: visibleMobileColumns(entity)
  does the same for mobile card fields, keeping inaccessible (🔒) cells.
- Verified live on the tickets project ticket_board: cards show only set
  fields.
