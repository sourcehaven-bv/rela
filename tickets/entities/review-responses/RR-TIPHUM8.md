---
id: RR-TIPHUM8
type: review-response
title: flowcfg depends on the run store for one type alias
finding: type RunID = flowrun.RunID made the flows.yaml loader depend on the storage contract.
severity: minor
resolution: RunID now lives in internal/flow; flowrun and flowcfg alias it and the flowcfg to flowrun arch-lint edge is gone.
status: addressed
---
