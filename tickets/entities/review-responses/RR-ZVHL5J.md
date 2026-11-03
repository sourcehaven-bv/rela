---
id: RR-ZVHL5J
type: review-response
title: Ordering test pins internals instead of behaviour
finding: "trigger_auto_test.go:214 TestAuto_WaitsForTriggerIntoSameType calls newRound, enqueue and waitsForTrigger directly. It would still pass with the deferred-post hole from the first finding. TestAuto_CloseOnEnter covers only the trivial same-round case.\tA refactor of the queue internals breaks this test even though the behaviour holds, while the real ordering bug passes it.\tReplace it with an end-to-end case through th.round that puts the trigger in cooldown or failure before the auto edge, and assert message order and channel state."
severity: nit
resolution: Replaced TestAuto_WaitsForTriggerIntoSameType with the end-to-end cooldown and retry tests above, asserting message order and channel state.
status: addressed
---
