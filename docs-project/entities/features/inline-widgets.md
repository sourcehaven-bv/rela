---
id: inline-widgets
status: draft
summary: A generic syntax for embedding editable values (numbers, enums, booleans, text) inline within entity markdown content.
title: Inline editable widgets in markdown content
type: feature
---

# Inline Editable Widgets

## Problem

Users want to create entities with structured, editable values embedded in free-form markdown content. Examples include:

- Workout logs with adjustable reps/weight/sets
- Checklists with completion toggles
- Reviews with inline ratings
- Inventory with quantity fields
- Any content mixing narrative text with discrete editable values

Currently, all editable data must be in frontmatter properties, which separates values from their context.

## Proposed Syntax

Use backticks with curly braces: `` `{...}` ``

### Numbers

```markdown
Weight: `{#50}` kg              # stepper, no bounds
Weight: `{#50|0..200}` kg       # stepper with range
Temp: `{#22.5|10..30|0.5}`      # with step size
Progress: `{#75%|0..100}`       # percentage (slider)
```

### Enums/Select

```markdown
Status: `{@draft|review|done}`           # first option is default
Status: `{@=review|draft|review|done}`   # explicit default with =
Priority: `{@low|medium|high}`
```

### Boolean/Toggle

```markdown
Complete: `{?yes}`              # checkbox, currently yes
Enabled: `{?no}`                # checkbox, currently no
```

### Text

```markdown
Notes: `{$}`                    # empty text field
Name: `{$John}`                 # text field with value
Code: `{$ABC-123|[A-Z]{3}-\d+}` # with regex validation
```

## Type Inference Summary

| Prefix | Type | Widget |
|--------|------|--------|
| `#` | number | stepper/slider |
| `@` | enum | select/dropdown |
| `?` | boolean | checkbox/toggle |
| `$` | text | text input |

## Examples in Context

### Workout Log

```markdown
## Session - 2024-01-15

**Bench Press**
- `{#50|20..200}`kg × `{#10|1..20}`reps × `{#3|1..5}`sets
- RPE: `{#7|1..10}`

**Squats**  
- `{#80|20..200}`kg × `{#8|1..20}`reps × `{#4|1..5}`sets
- RPE: `{#8|1..10}`

Feeling: `{@great|great|good|okay|bad}`
Fasted: `{?no}`
Notes: `{$}`
```

### Code Review Checklist

```markdown
## Review: PR #123

- `{?yes}` Tests pass
- `{?yes}` No linting errors
- `{?no}` Documentation updated
- `{?no}` Breaking changes documented

Complexity: `{@low|low|medium|high}`
Approve: `{?no}`
Comments: `{$Needs docs for the new API}`
```

### Inventory Item

```markdown
## Widget XL-500

In stock: `{#42|0..1000}`
Reorder at: `{#10|0..100}`
Location: `{@=warehouse-a|warehouse-a|warehouse-b|storefront}`
Discontinued: `{?no}`
```

## Implementation Considerations

### Parsing

- Extend markdown parser to detect `` `{...}` `` patterns
- Extract: type prefix, value, constraints (range/options/pattern)
- Track position in content for in-place updates

### Data Model

```go
type InlineWidget struct {
    Type       WidgetType // Number, Enum, Boolean, Text
    Value      any
    Min        *float64   // for numbers
    Max        *float64
    Step       *float64
    Options    []string   // for enums
    Pattern    *string    // for text validation
    StartOffset int       // position in content
    EndOffset   int
}
```

### Rendering (Data Entry UI)

- Replace widget syntax with HTML input elements
- Style to fit inline with surrounding text
- Compact widgets: small steppers, inline selects

### Persistence

- On save, update the markdown content in-place
- Preserve surrounding text exactly
- Update only the value portion within `{...}`

### Plain Markdown Fallback

In viewers that don't support this syntax, users see:
```
Weight: `{#50|0..200}` kg
```
Which is readable, if not pretty. The value (50) is visible.

## Open Questions

1. **Named widgets?** Should widgets have IDs for tracking/referencing?
   ```markdown
   `{#reps:10|1..20}` - named "reps"
   ```

2. **Slider vs stepper?** How to indicate preference? Perhaps `%` suffix for slider?

3. **Date/time widgets?** 
   ```markdown
   Due: `{~2024-01-15}`
   Time: `{~14:30}`
   ```

4. **Linked values?** Reference other widgets or properties?
   ```markdown
   Total: `{#=reps*sets}`
   ```

5. **Metamodel integration?** Define widget types/constraints in metamodel and reference by name?
   ```yaml
   # metamodel.yaml
   widgets:
     reps:
       type: number
       min: 1
       max: 50
   ```
   ```markdown
   Reps: `{:reps:10}`  # uses metamodel definition
   ```

## Alternatives Considered

### HTML in Markdown
```html
<input type="number" value="50" min="0" max="200">
```
Verbose, not readable in plain text, security concerns.

### YAML Blocks
```yaml
:::data
weight: 50
reps: 10
:::
```
Separates data from context, loses inline narrative flow.

### Frontmatter Only
Keep all data in properties. Doesn't support the mixed narrative + data use case.
