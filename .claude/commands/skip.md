# Skip Checklist Item

Skip a checklist item with a documented reason.

## Usage

Provide the checklist ID and item description: $ARGUMENTS

## Process

1. Fetch the checklist entity
2. Find the item in the markdown body
3. Ask for/confirm the skip reason if not provided
4. Update the item to strikethrough format:

   Before: `- [ ] API docs updated`
   After:  `- [x] ~~API docs updated~~ (N/A: no API changes)`

5. Save the updated checklist

## Validation

- The reason must be meaningful (not just "N/A" or "skipped")
- Common valid reasons:
  - `N/A: no API changes`
  - `N/A: documentation-only change`
  - `N/A: internal refactor, no public API`
  - `Deferred: will address in TKT-XXX`

Items without proper reasons will fail validation when marking the checklist as done.
