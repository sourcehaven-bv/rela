---
id: RR-UD2I
type: review-response
title: CheckboxWidget display uses emoji glyphs (no a11y)
finding: |
  '✓' (U+2713) and '☐' (U+2610). The empty-ballot-box rendering varies across system fonts -- on macOS Safari it's a fairly visible box, on some Windows fonts it falls back to a generic glyph or even tofu. No aria-label, no semantic state for screen readers. A screen reader user hears "check mark" or, worse, nothing for the empty box.
severity: significant
status: addressed
resolution: |
  CheckboxWidget display mode now renders <input type="checkbox" :checked disabled aria-readonly="true" class="display-checkbox" />. Native semantics for screen readers ("checkbox, checked|unchecked, read-only"), consistent rendering across OSes/fonts, no glyph fallback hell. CSS tweak makes the disabled checkbox visually distinct (opacity 0.85, cursor default) without losing the native affordance. The Known Behaviour Deltas in TKT-UD7YR updated -- the cards/list checkbox rendering is "disabled checkbox" not "✓ or ☐ glyph". Tests updated to assert input[type=checkbox] checked/disabled state instead of glyph text content.
---
