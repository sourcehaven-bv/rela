---
id: RR-GWVGDX
type: review-response
title: 'AC 3 is unimplementable: kanban emoji live inside user-authored label strings, not an icon field'
finding: |-
    The plan and AC 3 say the icon work 'replaces the emoji glyphs in the sidebar and kanban column headers'. The sidebar half is fine, but the kanban half is not implementable as specified.

    Kanban column emoji are literal characters inside user-authored `label:` strings in data-entry.yaml (prototypes/data-entry/project/data-entry.yaml:551-556):

        - value: open
          label: "\U0001F4E5 To Do"
        - value: in-progress
          label: "\U0001F527 In Progress"
        - value: resolved
          label: "✅ Done"

    There is no icon field on kanban columns. For the SPA to 'replace' these it would have to parse leading emoji out of arbitrary user-supplied label text and map them to components -- a lossy heuristic over user data, and a silent mutation of what an author explicitly typed.

    This also corrects a related observation from the original browser tour: the missing space in '\U0001F4E5To Do' is not a CSS/rendering bug. The label is authored with a space; the emoji-plus-space simply reads as run-together at that size. Nothing in the SPA is at fault.

    Three options: (a) drop kanban from AC 3 and scope icons to the sidebar only; (b) add a real optional `icon:` field to kanban column config and let the label be text (larger, but the correct model); (c) change only the prototype's YAML to drop the emoji, which fixes the demo but nothing for real users.

    Recommend (a) for this ticket with (b) as a follow-up, since (b) is a second config surface on top of the `span` one already being added.
severity: critical
resolution: 'Built the correct version rather than descoping. Option (b): KanbanColumn (config.go:396) gains an optional `Icon string`, and KanbanSwimlane (config.go:402) gains it too — the two structs share an identical Value/Label shape and both render labels, so icon-ing only one would leave the model asymmetric. The SPA renders `icon` beside the label and never parses emoji out of user `label:` text. The prototype YAML migrates its emoji from label into icon. Unknown icon names are a load-time config error (validateKanbans already iterates columns with indexed messages), with a frontend default fallback. Now AC 4. The ''missing space'' non-bug is recorded in the ticket so it is not re-reported.'
status: addressed
---

Discovered during design review by tracing where the kanban glyphs actually
originate. Verified at `prototypes/data-entry/project/data-entry.yaml:551-556`
and by confirming `KanbanView.vue` contains no emoji literals and no icon
handling — it renders `label` verbatim.
