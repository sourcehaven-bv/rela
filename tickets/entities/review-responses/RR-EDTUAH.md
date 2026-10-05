---
id: RR-EDTUAH
type: review-response
title: Picker fetches run serially per world and per type
finding: Extra per-world fetchAllList calls ran one after another for every target type; worst case T x (1+W) x 50 pages serially on every mount and inline create.
severity: significant
resolution: 'Target types load in parallel (Promise.all) and each type''s ambient and widened fetches start together. Skipping worlds that serve none of the type''s faces is not done: world-to-face data is only a hint (chains are server-side) and the list call for such a world returns empty cheaply.'
status: addressed
---
