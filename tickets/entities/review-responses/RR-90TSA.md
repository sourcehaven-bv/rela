---
id: RR-90TSA
type: review-response
title: extractInlines must skip TaskCheckBox node — state is captured separately
finding: extractInlineText currently skips *east.TaskCheckBox (markdown.go:629-630) because the checkbox state is captured by detectTaskCheckbox. extractInlines must do the same; otherwise task list items get a phantom inline at position 1.
severity: minor
resolution: extractInlines skips *east.TaskCheckBox — state continues to be captured by detectTaskCheckbox into the item table. Pinned in AC4.
status: addressed
---

# Finding

Existing policy at `markdown.go:629-630`:

```go
case *east.TaskCheckBox:
    // Skip: checkbox state is captured by the list-item extractor.
```

The new `extractInlines` must apply the same policy or task-list items will end
up with a phantom checkbox-typed inline as their first inline.

# Resolution

In `extractInlines`, skip `*east.TaskCheckBox` nodes (don't emit a Lua inline
for them). State is captured by `detectTaskCheckbox` and stored in the list-item
table's `task`/`checked` fields.

Add a test: `parse("- [x] foo")[1].items[1].inlines` does NOT contain a checkbox
inline.
