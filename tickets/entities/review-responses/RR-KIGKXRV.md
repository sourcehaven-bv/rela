---
id: RR-KIGKXRV
type: review-response
title: FLIP finish() throws on the infinite wobble animation
finding: measure() finished every animation from getAnimations(), including the infinite CSS wobble, which throws InvalidStateError.
severity: minor
resolution: 'useBoardFlip tracks the animations it started and finishes only those. Test: ''finishes only its own animations before it measures''.'
status: addressed
---
