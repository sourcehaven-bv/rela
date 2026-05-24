---
id: RR-3DRO
type: review-response
title: Click target detection misses label-mediated and text clicks
finding: event.target.closest('input[type="checkbox"][data-cb-idx]') only catches clicks on the input element itself. Clicks on the trailing text 'First' next to the checkbox (a natural affordance) do nothing. If marked or an extension ever wraps the input in a label, this selector quietly stops working.
severity: minor
reason: 'The narrow scope of this PR is restoring broken behavior (checkboxes that don''t toggle at all). Extending the click affordance to the trailing text label is a UX enhancement that benefits from a separate ticket: requires either wrapping the renderer output in `<label>` (which affects styling and the GFM-shape contract for downstream consumers) or extending the delegation handler to detect adjacent text. Will file a follow-up enhancement ticket; the BUG-N6WW scope stays at parity restoration.'
status: deferred
---
