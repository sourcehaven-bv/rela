---
id: RR-HLX2SFE
type: review-response
title: Failed rotation took the store lock on every send
finding: Below the hard cap, a rename that kept failing made every send take the store-wide lock only to fail again.
severity: minor
resolution: After a failed rename the feed waits until the journal grew by another rotation size (or reaches the hard cap) before trying again. TestHintFeed_FailedRenameTruncatesPastHardCap checks that the next send does not retry.
status: addressed
---
