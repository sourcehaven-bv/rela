---
id: RR-SYGQ4TR
type: review-response
title: Every preview logs flow warnings
finding: Validate runs compileFlows, which logs each flow warning, twice per preview or save.
severity: minor
resolution: The new check parses rules with triggers.Parse and flowcfg.Parse and starts no engine, so it adds no engine log lines. The existing Validate path still compiles flows once per preview, as rela validate does; no change there.
reason: Log noise only; changing compileFlows touches startup logging outside this ticket. Tracked in TKT-CB1YV7D.
status: addressed
---
