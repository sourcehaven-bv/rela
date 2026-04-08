---
id: RR-IA4B
type: review-response
title: 'F3: Unbounded AI spend from validation rules — no opt-in, no guardrail'
finding: Wiring AI into the validation runtime means any validation rule can ai.chat(...) on every entity on every analyze run. No quota, no cost warning, no opt-in flag, no per-project kill switch. A user who innocently drops ai.chat into a rule will wake up to an OpenAI bill or an ollama box at 100% CPU.
severity: significant
resolution: Resolved by un-wiring AI from internal/validation/lua.go. Documented in CLAUDE.md, PLAN-FOOU, and inline in validation/lua.go. AI-powered validations need their own design that addresses cost guardrails, per-rule opt-in, and per-rule budget — tracked as a follow-up ticket.
status: addressed
---
