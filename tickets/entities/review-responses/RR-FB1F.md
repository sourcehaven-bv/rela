---
id: RR-FB1F
type: review-response
title: 'L1: Drop WidgetMode widening entirely — pass no transitions instead'
finding: |
  Widening `WidgetMode` to `'display' | 'edit' | 'inline-edit'` introduces a third state across the entire widget contract for ONE behavioural delta: SelectWidget's transitions hint panel. The leverage: just don't pass `transitions` from SectionEditForm; SelectWidget's panel renders only when `hasTransitions && mode === 'edit'`, so absent transitions = no panel. Same outcome, far smaller blast radius. ACs 1, 2, 9 vanish; per-widget round-trip tests vanish (existing edit-mode tests cover SectionEditForm's usage); no UD7YR comment removal needed.
severity: significant
status: addressed
resolution: |
  Adopted. PLAN AC 1 deleted entirely. AC 2 reduced to "SectionEditForm passes `mode='edit'` (no widget changes) and does NOT pass `transitions` to SelectWidget; absence-of-panel is verified by an inline-edit-mode rendering test on SelectWidget with no transitions prop." `WidgetMode` stays at `'display' | 'edit'`. The UD7YR "reserved for IHCY7" comment is left in place (it's harmless and accurate forward-looking).
---
