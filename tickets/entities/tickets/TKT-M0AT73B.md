---
id: TKT-M0AT73B
type: ticket
title: List toggle widget
kind: enhancement
priority: medium
effort: m
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

## Description

A list column can render a boolean-like property as a checkbox or switch that
the user flips in place.

```yaml
columns:
  - property: status
    widget: toggle        # or switch
    on: done
    off: open
    other: disabled       # needed when the enum has more values: disabled | hidden | on
```

- `on` and `off` must be valid values of the property. A boolean property
needs neither (true/false).
- An enum with values other than `on` and `off` must declare `other:`, or
the config fails to load. `disabled` shows the box locked, `hidden` shows
nothing, `on` shows it checked.
- A flip is a normal PATCH through the field permission and transition
checks. Writability comes from the row's `_fields`, which the list computes once
per page (primed verdicts), not per row.
