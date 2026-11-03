---
id: RR-LFF8A0
type: review-response
title: Optional boolean sends absent or false for the same visual state
finding: 'RlChannelComposer.vue:116-121 and RlChannelField.vue checkbox branch. An optional boolean seeds as undefined (sent absent) but becomes false once checked and unchecked (sent false). The checkbox looks the same in both cases, but the stored message differs. Fix: either seed every boolean as false, or map false back to undefined for optional booleans. Write down which one the server expects.'
severity: minor
resolution: Unchecking an optional boolean now emits undefined, matching the app rule that an unset optional box is absent, and this is documented in RlChannelField (story OptionalBoolean).
status: addressed
---
