---
id: RR-MOKG4FT
type: review-response
title: Decisions in a working state lose reply_to
finding: "onDecide read the cause from decisionSeq. In a working state with a quiet cause, decisionSeq is now undefined, so a header decision was posted without reply_to. That changes the thread on the server, not only where the buttons show."
severity: significant
resolution: "Split causeSeq (the cause, for reply_to) from decisionSeq (where the buttons go). onDecide replies to causeSeq and focuses the row only when placed on it. Test: a header decision still replies to the quiet cause."
status: addressed
---
