---
id: RR-9EAUGR
type: review-response
title: pendingCardChanges not cleared — incoming relation silently written to the next record
finding: An incoming-direction RelationPicker renders in create mode (FormFieldList.vue:91 has no entityId gate) and routes its selection through updateIncomingPicker into pendingCardChanges, NOT into `relations`. The reset cleared `relations` and remounted the widget but left that map, so record N's incoming relation was written to record N+1 with the user touching nothing. A code comment in handleSubmit asserted the map is always empty in create mode, which was false.
severity: critical
resolution: 'Added pendingCardChanges.value.clear() to resetCreateForm and corrected the false comment in handleSubmit to state that RelationCards never render in create mode but an incoming RelationPicker does, and both share the map. Pinned by a new unit test whose picker stub actually EMITS incoming-changed (the previous stub only counted mounts, so it validated the remount while being structurally unable to exercise the leaking state path). Mutation-verified: removing the clear() fails the test.'
status: addressed
---
