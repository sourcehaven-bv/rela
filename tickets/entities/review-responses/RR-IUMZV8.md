---
id: RR-IUMZV8
type: review-response
title: Silent span clamping contradicts the codebase's strict load-time config validation convention
finding: |-
    The plan specifies that invalid `span` values (0, 13, -1, 'abc', fractional) 'clamp/ignore to full width. Degrade, never error.' That contradicts how this codebase treats config.

    dataentryconfig.ValidateConfig (internal/dataentryconfig/validate.go:120) runs a strict two-phase validation at load: phase 1 rejects unknown keys -- with did-you-mean suggestions for known typos -- and phase 2 runs ~13 semantic validators (validateViews, validateForms, validateKanbans, ...) that aggregate error strings into a ConfigValidationError. The project's stance is that a malformed data-entry.yaml fails loudly at startup, not silently at render time.

    Under the planned behaviour, `span: 13` renders full width with no diagnostic anywhere. The author sees a layout that silently ignores what they wrote and has nothing to grep for -- exactly the failure mode checkUnknownKeys exists to prevent.

    The two rules should be separated:

    - Load time: `validateViews` rejects a span outside 1..12 with a clear message, consistent with every other config validator.
    - Render time: the frontend still defends -- a value arriving out of range (older config, hand-crafted API response) falls back to full width rather than emitting broken CSS.

    Note a non-integer such as `span: "abc"` never reaches a validator: it fails yaml unmarshal into an int field first, producing a less friendly parse error. Worth a test to pin the actual message.

    The CLAUDE.md rule 'no silent failures (errors must be surfaced, not just logged)' applies directly.
severity: significant
resolution: 'Plan corrected to match the project convention. Spans outside 1..12 are now a load-time config error emitted by validateViews/validateForms/validateKanbans with an indexed message, consistent with the rest of ValidateConfig (validate.go:120). The frontend still independently falls back to full width for out-of-range values arriving from hand-crafted API responses, so defence-in-depth is kept without the silent-ignore behaviour. Now AC 7. A test will pin the yaml unmarshal error message for a non-integer span such as `span: "abc"`, which fails before any validator runs.'
status: addressed
---

Verified against `internal/dataentryconfig/validate.go:120-147` (the
`ValidateConfig` phase structure) and `checkUnknownKeys` at line 150, which goes
as far as suggesting corrections for typo'd keys — evidence that loud config
diagnostics are a deliberate project convention, not an accident.
