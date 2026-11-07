---
id: RR-QJG4IOH
type: review-response
title: Typed run id and caller clock on Create
finding: flowcfg.RunID exists; CreatedAt should come from the caller.
severity: nit
resolution: flowrun.RunID with flowcfg.RunID as an alias of it; NewRun carries At.
status: addressed
---
