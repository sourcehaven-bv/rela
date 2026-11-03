---
id: RR-1ZMWZH4
type: review-response
title: resetServer events may leak into the next stream
finding: The deletes and probe write in resetServer happen just before the stream opens; their batched event could reach it.
severity: minor
reason: Not reproduced in any run (about 1500 runs this session). The stream opens after resetServer returns and waits for ready; if it shows up, the first segment would show it and the fix is a quiet wait before connecting.
status: deferred
---
