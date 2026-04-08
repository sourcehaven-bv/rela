---
id: RR-Z5XJ
type: review-response
title: temperature=0 cannot be distinguished from unset; ships a known correctness bug
finding: 'Plan accepts that ChatRequest.Temperature float64 zero-value collides with ''unset'', so temperature=0.0 will not be sent upstream. Documents ''use 0.0001 as workaround''. This is a real correctness bug — temperature=0 is the most common deterministic-sampling setting (golden tests, evals, reproducible runs). Users will silently get non-deterministic output that looks deterministic. Fix: use *float64 (or equivalent optional type) end-to-end, including for MaxTokens (where 0 is also semantically distinct). The Lua side already knows whether the key was present in the table — propagate that. ~30 minutes of extra work; removes a footgun forever.'
severity: critical
resolution: 'ChatRequest.Temperature is now *float64, MaxTokens is *int. JSON tags use omitempty so nil pointers are absent from the wire. Lua binding uses LGetField + LNil check to distinguish ''key absent'' from ''key set to 0''. AC #15 and AC #30 verify temperature=0 is sent distinctly from absent at both Provider and Lua layers.'
status: addressed
---
