---
id: RR-82JF79Q
type: review-response
title: 'Design: saves that do not know branches drop them'
finding: Run copies state fields one by one in FromNew and both backends, and Engine.save builds State from scratch, so release, backoff, claim, rest and mark would drop branches.
severity: significant
resolution: 'Plan updated: Run embeds flowrun.State; backends and save start from run.State and change only what they mean to. A conformance test round-trips every State field.'
status: addressed
---
