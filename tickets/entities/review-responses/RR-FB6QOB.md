---
id: RR-FB6QOB
type: review-response
title: Dialog danger tied to button style
finding: 'The edge case at plan lines 543-545 makes the dialog''s confirm button follow the view''s danger, so style: primary or style: default into a cancelled final state gives a plain confirm button on a dialog that says the session ends and cannot be undone. Decide explicitly whether the dialog tone describes the outcome (derived from the target) or the author''s style, and write it in AC 7. Either is defensible; the plan should not reach it by accident.'
severity: nit
resolution: 'Decided in AC 8 and Q6: the dialog confirm follows the outcome, red when the target ends cancelled or failed or when style is danger. outcomeDanger keeps the dialog red when an authored style removed a derived danger.'
status: addressed
---
