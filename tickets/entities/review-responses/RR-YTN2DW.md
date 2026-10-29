---
id: RR-YTN2DW
type: review-response
title: Busy field label default is not computable where the plan puts it
finding: Step 6 makes the label default "the channel type's label, else busy". But KanbanCardField.DisplayLabel (internal/dataentryconfig/config.go:1531) and the SPA's cardFieldLabel (frontend/src/types/config.ts:594) have no access to realtime labels; ChannelSchema exposes no label. The plan should either add the label to ChannelSchema and resolve it at load, or take it in the SPA from the channel descriptor. It should also state what show_label does on a busy field.
severity: minor
resolution: The label default is the channel type name, computable in Go and the SPA without realtime, as property fields default to the property name (DEC-6C1NAA); show_label works as on other fields. The realtime type label travels in each _busy entry as type_label for the accessible name.
status: addressed
---
