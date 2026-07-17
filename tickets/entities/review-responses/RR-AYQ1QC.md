---
id: RR-AYQ1QC
type: review-response
title: No test pinned untouched-create picker emitting nothing
finding: The fix's core promise is that an incoming picker left untouched on create emits nothing (no spurious data:[] wipe). buildRelationsPatch's skip is tested in isolation, but no test asserted that mounting an incoming picker in create mode and never touching it produces zero incoming-changed events.
severity: minor
resolution: 'Added unit test ''create mode: an untouched incoming picker emits nothing'' in RelationPicker.test.ts asserting emitted(''incoming-changed'') is falsy after mount with no interaction.'
status: addressed
---
