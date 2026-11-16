---
id: RR-AJ1WG1F
type: review-response
title: Triggers-only flows.yaml would never be parsed
finding: compileFlows returns nil unless DeclaresFlows; widening DeclaresFlows would add _flow-run, the flow engine, run store and outside-edits ceiling to projects without flows.
severity: significant
resolution: 'Plan: separate flowcfg.DeclaresTriggers and a base.triggers field; triggers parse without an ACL policy and do not switch on the flow engine.'
status: addressed
---
