---
id: RR-9RQJUB
type: review-response
title: 'Design: tagging inside a store.Tx misbehaves'
finding: pg would tag pre-write state; sqlite risks a writer deadlock.
severity: significant
resolution: The store refuses tag calls inside a Tx; documented for automation Lua (R6).
status: addressed
---
