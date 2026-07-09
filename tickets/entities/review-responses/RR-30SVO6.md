---
id: RR-30SVO6
type: review-response
title: Kanban columnTitle label!==value heuristic can override explicit config label
finding: KanbanView columnTitle treats column.label===column.value as 'no explicit label' and falls through to the enum label, so an author who deliberately sets label equal to the value gets it overridden. Narrow, display-only edge; the config shape can't distinguish 'absent' from 'present-equal-to-value' without a change. Known limitation.
severity: nit
reason: Display-only, extremely narrow edge (author explicitly sets a kanban column label equal to the raw value AND wants the raw value shown despite an enum label existing). Cannot be distinguished from 'no label' without a config-shape change (an explicit null/absent marker), which is out of scope for this ticket. Documented as a known limitation in the KanbanView columnTitle comment.
status: deferred
---
