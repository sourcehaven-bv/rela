---
id: RR-VD9L
type: review-response
title: Server-side input validation absent on index; negative indices accepted
finding: strconv.Atoi(indexStr) accepts negatives. Out of scope for this PR but adjacent — the PR makes the API reachable from the SPA, so the existing gap is now exercised by real traffic.
severity: nit
reason: Out-of-scope for BUG-N6WW (a client-side fix). The server-side index-bounds gap pre-exists in `internal/dataentry/handlers.go:49` and applies to every code path that hits `/api/toggle-checkbox`, not just this PR's traffic. Will file a separate hardening ticket for the server-side validation — fixing it here would conflate two concerns and require Go test changes for a frontend-shape bug.
status: deferred
---
