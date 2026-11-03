---
id: RR-SP324Q
type: review-response
title: Nodes disagree for up to one tick
finding: With reload, a run queued by one node can reach a node that has not reloaded yet.
severity: minor
resolution: Documented; the run fails once and the retry ladder retries it; CreateRun still prevents double runs.
status: addressed
---
