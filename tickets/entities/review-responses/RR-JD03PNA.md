---
id: RR-JD03PNA
type: review-response
title: Every API GET takes the hub mutex
finding: servedTargets took h.mu for each GET under /api/ so ordinary reads queued behind commitRound.
severity: significant
resolution: byFP moved under its own fpMu. A principal with no stream costs one map look under fpMu and no parse; only a GET with a matching subscription takes h.mu to record.
status: addressed
---
