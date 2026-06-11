---
id: RR-13LR
type: review-response
title: parseInt fallback '0' masks programmer error if invariant changes
finding: parseInt(checkbox.dataset.cbIdx || '0', 10) — if dataset.cbIdx is missing (currently impossible given selector requires [data-cb-idx]), this silently toggles index 0. Dead-defensive code that masks a real bug if invariants change.
severity: nit
resolution: Replaced `parseInt(checkbox.dataset.cbIdx || '0', 10)` with explicit `if (raw === undefined) return; const idx = parseInt(raw, 10); if (Number.isNaN(idx)) return`. Missing or malformed attribute now fails closed instead of silently toggling index 0.
status: addressed
---
