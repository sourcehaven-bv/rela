---
id: RR-GHMVDTA
type: review-response
title: Double trigger can start two batch runs
finding: pending was set only after the confirm, so a second click while the confirm was open started a second run.
severity: minor
resolution: useBatchAction holds a starting flag from the first click to the end of the run; a test pins one run per click.
status: addressed
---
