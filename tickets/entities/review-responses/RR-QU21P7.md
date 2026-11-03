---
id: RR-QU21P7
type: review-response
title: No conformance test for a reply that moves the session
finding: 'channelstest/session.go: no backend test covers a step on a reply. The rev order is message, then parent reply_count, then notice: SessionNotice doc in channels/session.go, filechannels ops.go moveSession after the parent bump, and sqlchannels insertMessage returning the parent rev. Nothing tests it, and nothing tests a parent in another segment than the message and notice. TestSessionPost_Reply checks only the state. Fix: add testSessionReply to RunAll. A reply in segment 2 to a parent in segment 1 with a move should give message rev r, parent rev r+1 and notice rev r+2, all returned by an AfterRev page.'
severity: minor
resolution: testSessionReply in the conformance suite checks message, parent and notice revision order; a file test covers a parent in another segment.
status: addressed
---
