---
id: RR-919QPZ
type: review-response
title: SweepDeps cannot mark Local and NewSweeper skips nil validation
finding: B6 and A3 say sweep transitions call local.mark, but SweepDeps has no Local or marker field. NewSweeper returns *Sweeper with no error, against the constructor rule. Add a narrow marker interface and return an error on nil Store, Config, Anchors or Now.
severity: minor
resolution: SweepDeps gains a narrow marker interface and NewSweeper returns an error on any nil dependency.
status: addressed
---
