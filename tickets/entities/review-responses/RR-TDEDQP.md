---
id: RR-TDEDQP
type: review-response
title: Q3 no-inheritance contradicts how the key will be read and forces repetition
finding: 'Q3 (plan lines 633-636) gives each button one source. Consequence: an operator writes abandon with style danger at type level, later adds a preset "Abandon (no reason)" of type abandon, and the preset is silently not red; an Allow type with a confirm text loses the question on an "Allow once" preset. That is the 3 AM surprise the plan says it avoids. A per-key fallback (preset key, else type key, else derived) is three lines and fully predictable, and the plan already has the explicit opt-outs it needs (style: default, confirm: false). Resolve the fallback at compile time in realtimecfg so TransitionButton.Options is already merged; the SPA then still reads exactly one source per row (tr.button) and AC 6''s "preset options do not reach the type button" stays true. Change Q3 to inherit per key, and replace the test "type options do not reach a preset" with "a preset inherits each unset type option" and "a preset''s style default overrides the type''s danger".'
severity: significant
resolution: 'Q3 now inherits per key: preset, then decision_button, then the derived rule, merged at load by typeButtonChecker so TransitionButton.Options is resolved. New AC 2 with TestButtonOptions_Inherit covers inheritance and the style default and confirm false opt-outs.'
status: addressed
---
