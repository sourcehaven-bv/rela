---
id: RR-IQZGRZP
type: review-response
title: Locked hidden row reported as inaccessible
finding: pre.check ran after the IsLocked refusal, so a hidden locked row was not reported as not found.
severity: minor
resolution: patchEntityOnce runs pre.check before the locked check.
status: addressed
---
