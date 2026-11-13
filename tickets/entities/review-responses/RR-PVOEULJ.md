---
id: RR-PVOEULJ
type: review-response
title: Unset reads as initial contradicts current behaviour
finding: flow.Definition documents an unset instance as reading as the initial state, but statemachine treats an unset property as an empty value that selects no branch, so the write is illegal.
severity: significant
resolution: 'Addressed in the plan: decideMove refuses an empty from before flow.Read, keeping today''s behaviour; the package doc states that a headless flow does not read unset as initial; a test pins unset to the initial successor as 422; the question is recorded in M5c''s scope.'
status: addressed
---
