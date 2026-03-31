---
id: RR-TVBX
type: review-response
title: Test pollution via package-level variables
finding: Tests directly mutate package-level global variables (ws, out, validateChecks, quiet) without cleanup. These tests will flake in parallel execution.
severity: critical
resolution: Added t.Cleanup() to all tests that modify global state, restoring origWs, origOut, origChecks, origQuiet after each test.
status: addressed
---
