---
id: RR-GV1RVL
type: review-response
title: Kept relation loses its pickerTypes across the reset (not reachable in production)
finding: 'Review reported that resetCreateForm clears pickerTypes while re-applying kept relation ids, so reshapeLegacyToModern returns null and the SECOND save aborts with ''unknown types, reload the form'' while the widget still shows the peer — a silent lost record. Verified the mechanism is real, but NOT reachable in production: the reset bumps saveGeneration, which remounts RelationPicker, and its onMounted re-emits update:types for any pre-existing outgoing selection (RelationPicker.vue:352-354) precisely to cover this. The reviewer''s repro used a stub that never emits on mount. Confirmed by e2e: removing the carry-over keeps the suite green and both records carry the relation.'
severity: minor
resolution: Kept the pickerTypes carry-over anyway — it makes the reset self-sufficient rather than dependent on a child component's mount timing, and the failure mode if that dependency ever broke is a lost record behind a misleading 'reload the form' message. The comment now states plainly that it is belt-and-braces and records the verification, rather than implying it guards something currently reachable. Severity downgraded from critical to minor on that evidence. A unit test pins the behaviour (mutation-verified against the stub path); a new e2e test covers a kept relation end to end, which was the real coverage gap.
status: addressed
---
