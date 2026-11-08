---
id: RR-3HVBDE2
type: review-response
title: 'Design: peer key detection must not be stricter than verification'
finding: Only the relapeer_ prefix after Bearer stripping may decide the peer path; several Authorization values need handling.
severity: minor
resolution: Detection reads every Authorization value, strips Bearer like stripBearer and tests the prefix only. More than one value with a peer key answers 401. Length and alphabet are checked in Verify.
status: addressed
---
