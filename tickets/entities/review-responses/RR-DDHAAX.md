---
id: RR-DDHAAX
type: review-response
title: SetLeases ordering and nil gaps
finding: SetLeases after StartWatching never wakes the hub; a late call races; typed nil passes the nil check
severity: minor
resolution: 'Fixed in the review-fix commit (Lease HTTP: review fixes) with tests where behaviour changed.'
status: addressed
---
