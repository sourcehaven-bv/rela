---
id: RR-TYONDR4
type: review-response
title: A failed write leaves the step claimed
finding: If PatchEntity failed after Claim the claim stayed for ClaimTTL. Other holders got 409 and the same principal could keep the step locked by failing repeatedly.
severity: minor
resolution: complete calls the new Engine.ReleaseClaim on a write failure. It releases only the caller's claim at that generation. TestEngine_ReleaseClaim pins it.
status: addressed
---
