---
id: RR-W6ZLXZ7
type: review-response
title: Waiting for a dirty round lets hidden writes time the frame
finding: Comparing a served record at whichever round next ran made the frame land on the round a hidden write triggered. A client that forces a mismatch (for example a 403 from the host gate) would get a frame per hidden write.
severity: critical
resolution: The next tick always replays a recorded mismatch (pending) so timing follows the client's own GET. Records made after a round collected wait for the next round (servedState.gen). recordServed sits below the transport gates so their refusals are never recorded. A hash a replay already framed for is not recorded again until the baseline is served (suspect).
status: addressed
---
