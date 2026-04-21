---
id: RR-2R1HG
type: review-response
title: Acceptance criteria don't cover read-only 'prefix picker' state for single-prefix
finding: 'Criterion 1 says ''multi-prefix types show picker''. Criterion 2 says ''single-prefix types do NOT show a picker''. But there''s a third state — types with `id_type: manual` (which have prefixes too, per the validate test at metamodel/validation_test.go). Should the picker show for manual-ID types? Answer is no (the ID is fully user-supplied), but the plan doesn''t state that explicitly. Add to plan: ''prefix picker is only shown when id_type != manual AND id_prefixes.length > 1.'''
severity: minor
resolution: Plan's acceptance criteria and composable spec now guard picker with `id_type !== 'manual' && id_prefixes.length > 1`. Edge cases section covers the manual-with-declared-prefixes case.
status: addressed
---
