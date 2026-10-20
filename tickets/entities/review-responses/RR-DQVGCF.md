---
id: RR-DQVGCF
type: review-response
title: Edit mode cannot handle a restricted ref
finding: 'RlChannelComposer.vue:175-185 and RlChannelField.vue:139-155, types.ts ChannelEditing (lines 270-277). Plan R7 says edit is offered on messages holding a ref the caller cannot read, and the server keeps that ref when the request omits it. ChannelEditing has no way to say a property holds a restricted ref, so the composer treats it as empty. (1) A required ref property that is restricted fails check() with "Required." and the edit can never be saved. (2) An optional restricted ref renders as an empty "Choose…" button, which tells the user the value is unset and invites them to overwrite something they cannot see. (3) The emitted event lists the property in `rendered` with value undefined, so the app cannot tell "user cleared it" from "restricted, untouched". Fix: add `restricted?: string[]` (or a ChannelRef map) to ChannelEditing. For those properties, render the restricted RlRefChip with a "Replace…" button, skip the required check, and leave the property out of `rendered` and `values` unless the user picks a replacement. Add an Edit story with a restricted required ref that saves.'
severity: significant
resolution: ChannelEditing gained restricted, and those ref fields now show the restricted chip with a Replace button, skip the required check and stay out of values and rendered unless replaced (story EditRestrictedRef).
status: addressed
---
