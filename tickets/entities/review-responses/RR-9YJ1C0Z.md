---
id: RR-9YJ1C0Z
type: review-response
title: Harness name and arch-lint rules
finding: flowtest reads as a test package for internal/flow.
severity: nit
resolution: Package is flowrun/flowruntest; arch-lint keeps flowrun a leaf and adapters reachable only from tagged recipes.
status: addressed
---
