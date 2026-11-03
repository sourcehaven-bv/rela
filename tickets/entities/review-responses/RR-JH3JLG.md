---
id: RR-JH3JLG
type: review-response
title: Restamp sits on Opener with a dummy flag
finding: provision built a new Opener per call with verifiedIdentity=false, which Restamp never reads.
severity: minor
resolution: Restamp is now a package function access.Restamp(ctx, policy, sub) that rejects a nil policy; provision calls it directly. TestRestamp_RejectsNilPolicy pins the nil case.
status: addressed
---
