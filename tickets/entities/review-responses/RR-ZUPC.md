---
id: RR-ZUPC
type: review-response
title: 'F1: Validation runtime builds a fresh AI provider per rule per entity'
finding: validate() is called once per entity per rule. LoadProvider re-reads and re-parses .rela/ai.yaml from disk every single call, re-runs URL validation, and re-constructs an http.Client. On a project with 200 entities and 10 rules, that's 2000 file reads and 2000 http.Client allocations per analyze run.
severity: significant
resolution: Resolved by un-wiring AI from internal/validation/lua.go entirely. AI in validation rules needs its own design (per-rule opt-in, cost guardrails, longer per-rule budget) and is tracked as a follow-up ticket.
status: addressed
---
