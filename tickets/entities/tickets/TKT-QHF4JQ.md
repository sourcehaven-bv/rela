---
id: TKT-QHF4JQ
type: ticket
title: Recursive unknown-key validation for nested data-entry config structures
kind: enhancement
priority: medium
status: backlog
---

## Problem

`checkUnknownKeys` (`internal/dataentryconfig/validate.go`) unmarshals the raw
YAML into `map[string]any` and checks only **top-level** keys against
`validTopLevelKeys`. Nested structures are never structurally validated, and
`yaml.Unmarshal` is called without `KnownFields(true)`, so an unknown nested key
decodes with `err == nil`.

A typo like `visible_wen:` or `clear_when_hiden:` on a form field is therefore
silently ignored. The author sees no error and reasonably assumes the setting
took effect.

## Why it matters

Surfaced by BUG-FB0LN8's design review (RR-O0KRI2). That bug added
`clear_when_hidden`, whose *absence* selects a behavior — so a typo silently
selects the other one. The enum itself is now allowlist-validated, which covers
a bad **value**, but not a misspelled **key**.

The same shape applies to every nested key in the config: `visible_when`,
`required_when`, `readonly`, list column options, view section fields.

## Scope

Recursive unknown-key checking for nested config structures (`forms[].fields[]`,
`lists[].columns[]`, `views[].sections[]`, `kanbans[].card.fields[]`, …), with
the same did-you-mean suggestion treatment the top-level check already gives.

Deliberately NOT folded into BUG-FB0LN8: this would start rejecting configs that
parse cleanly today, which is its own compatibility surface and deserves its own
review rather than riding along with a critical data-loss fix.

## Notes

- Prefer deriving the valid key set from struct tags (as
`TestValidTopLevelKeysMatchConfigStruct` already does for the top level) rather
than hand-maintaining a second list that can drift.
- Consider whether this should be an error or a warning on first release, given
existing configs in the wild may carry stray keys today.
