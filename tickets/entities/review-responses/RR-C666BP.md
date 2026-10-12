---
id: RR-C666BP
type: review-response
title: Copy piles instead of schedulerstate
finding: 'Architecture design review: kvstate is single-process; piles/kvpiles is the matching precedent.'
severity: significant
resolution: Plan modelled on internal/piles (kvpiles, pgpiles, pilestest, appbuild/piles.go).
status: addressed
---
