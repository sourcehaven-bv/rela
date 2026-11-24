---
id: BUG-R7K2M4Q
type: bug
title: TestSSEACL_Debounce flakes on a loaded runner
description: The debounce test read the sink after a fixed 80 ms slack and saw 0 frames when the change feed was slow, which failed the v26.11.6 release.
priority: medium
effort: xs
why1: The test read the sink before the first frame arrived.
why2: It waited a fixed flush window plus 80 ms.
why3: The event travels through a goroutine and the change feed before the loop sees it.
why4: A loaded CI runner delays that path past the fixed wait.
why5: Timing tests used sleeps instead of waiting for the observable outcome.
prevention: The test waits for the first frame, then one more window to prove no second frame follows.
completed: "2026-11-24"
status: done
---

The v26.11.6 Release workflow failed on TestSSEACL_Debounce (0 frames).
Fixed by waiting for the first frame before the settle window.
