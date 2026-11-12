---
id: RR-I4FL593
type: review-response
title: Runs whose writes always fail can starve the rest
finding: A run whose backoff write fails keeps its past due time and stays at the front. The tick cursor reset to the front every tick, so 256 such runs would starve all others.
severity: minor
resolution: The worker keeps the cursor for the next tick when the last round was full, and starts from the front once a round is not full. TestFlowWorker_NextTickResumes.
status: addressed
---
