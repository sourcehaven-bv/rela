---
id: FEAT-GE1YY
type: feature
title: 'Backend CI quality gates: coverage floors, govulncheck, gosec'
summary: 'Idiomatic Go quality signals in CI: package-level coverage floors (no per-file ratchet), govulncheck on every PR, gosec on security-sensitive packages'
description: The Go backend needs CI quality gates that match Go community norms rather than frontend-style per-file coverage ratchets. This feature covers package floor thresholds via go-test-coverage, blocking govulncheck on every PR, and gosec-based security scanning of the AI/network/Lua layers.
priority: medium
status: proposed
---
