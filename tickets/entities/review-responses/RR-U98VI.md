---
id: RR-U98VI
type: review-response
title: :deep(input) in .checkbox-wrapper is too broad
finding: FieldShell .checkbox-wrapper :deep(input) sizes ANY input 18x18px. Today only the checkbox renders in that slot, but a future widget using labelPosition='after' (perfectly reasonable for view-mode) would get its inputs miniaturized. Forward-looking footgun.
severity: significant
resolution: 'FieldShell.vue: tightened to .checkbox-wrapper :deep(input[type=''checkbox'']). Future widgets using labelPosition=''after'' no longer get their inputs miniaturised to 18x18px.'
status: addressed
---
