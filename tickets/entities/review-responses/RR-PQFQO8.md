---
id: RR-PQFQO8
type: review-response
title: Small inaccuracies in the hub description
finding: Step 6 says "hasPending keeps the ticker running", but the ticker always runs; hasPending only makes a tick run a filtered round. The rename of SubscribeReads also touches test callers in internal/realtime and internal/dataentry, which the files-to-modify list leaves out.
severity: nit
resolution: Corrected the hasPending description (the ticker always runs; hasPending only makes a tick run a filtered round) and added the realtime and dataentry test callers of SubscribeReads and channelChanges fakes to the files list.
status: addressed
---
