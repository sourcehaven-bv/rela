---
id: RR-BY5NQTX
type: review-response
title: '[security] Two distinct served responses force a replay and frame each tick'
finding: '[security] With mixed set no suspect hash was kept so a client getting two different responses for one live URL could cause a replay and frame every tick.'
severity: nit
resolution: Mixed frames now count toward servedState.streak so they stop after liveServedStreak frames like any other mismatch.
status: addressed
---
