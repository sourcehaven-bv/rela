---
id: RR-V0FK61
type: review-response
title: AC3 contradicts handleSSE ordering
finding: 'Acceptance criterion 3 says an expiry already in the past ends the stream "without writing data frames". The plan arms the timer only in runSSELoop. Before runSSELoop starts, handleSSE has already called a.live.register and written the first frame (ready, or refresh with reason resync), which carries the conn id and a resume token. A test written to the criterion will fail. Either reword AC3 to "ends right after the first frame, and the live connection is unregistered", or check credentialExpiry in handleSSE before broker.subscribe and live.register and return there. The second option is cleaner: a stream that is already dead never gets a conn id.'
severity: minor
resolution: 'Plan updated: handleSSE checks the expiry before the broker subscription and live.register; AC3 now asserts no frames and no hub connection.'
status: addressed
---
