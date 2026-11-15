---
id: RR-RG4O28T
type: review-response
title: Nav status store path had no differential against the Go path
finding: 'cranky: no test compared navStatusInStore with the Go loop: first-match order, a denied rule, a hidden-field fallback and an empty list were unpinned.'
severity: significant
resolution: 'Addressed: navStatusInGo split out, and TestNavStatus_InStoreMatchesGoPath compares both paths per principal and entry and asserts which path handled it.'
status: addressed
---
