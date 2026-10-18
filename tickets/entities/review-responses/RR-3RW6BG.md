---
id: RR-3RW6BG
type: review-response
title: Per-identity ephemeral cap blocks a net-zero overwrite
finding: add checks the cap and byte budget before the own-overwrite path, so a user at the cap cannot replace their own signal.
severity: minor
resolution: The poster's own replacement is computed before the count and byte checks, so an overwrite is net zero.
status: addressed
---
