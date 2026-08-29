---
id: RR-2VFFCD
type: review-response
title: Error message listing ~150 valid icon names is unusable at the terminal; the plan inherits a message design sized for 16
finding: |-
    `validateIconName` (config.go:392) formats every unknown name as `"%s: unknown icon %q (valid: %s)"` with `strings.Join(sortedMapKeys(ValidIconNames), ", ")`. At 16 names that is a readable one-liner. At ~150 it is a ~1,600-character wall of comma-separated words, repeated once per offending entry — a config with five typo'd icons emits ~8 KB of near-identical text.

    RR-GTOQCF's resolution made this message "the authoritative reference" for discovering names and removed the docs list specifically because the message "cannot go stale". This ticket restores the docs table, which changes the message's job: it no longer needs to be a catalogue, because there is now a real catalogue. The plan does not revisit the message at all — it keeps a design chosen under the old constraint while removing the constraint.

    Also note AC 7 requires "error text contains `none`", which with a 150-name join is technically satisfiable but useless to a human scanning the output.
resolution: |-
  Addressed in plan (Approach §7). The unknown-icon message becomes diagnostic rather than exhaustive: nearest-match suggestion, explicit `none` mention, and a pointer to `docs/data-entry.md#icons`. AC 7 restated to assert the suggestion rather than "contains none".
severity: minor
status: addressed
---

## Recommended fix

Make the message diagnostic rather than exhaustive:

1. **Suggest, don't enumerate.** The codebase already does typo-suggestion —
`ValidateConfig` "even suggests corrections for typo'd keys" per the
`validateSpan` doc comment. Reuse that: nearest-match on the unknown name.
2. **Point at the catalogue.** `see docs/data-entry.md#icons for the full list`.
3. **Always name the opt-out**, since it is not discoverable by similarity:
`(or "none" for no icon)`.

Something like:

```
navigation "My Tickets": unknown icon "inbxo" (did you mean "inbox"?);
use "none" for no icon, or see docs/data-entry.md#icons for all 152 names
```

That is shorter than today's message, more actionable, and degrades gracefully
as the set grows.

Restate AC 7 to assert the *suggestion* and the `none` mention rather than
"contains none", which a 150-name join satisfies vacuously.
