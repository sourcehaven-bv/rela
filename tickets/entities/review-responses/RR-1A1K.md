---
id: RR-1A1K
type: review-response
title: handleCommandExec accepts GET — bypasses any non-safe-method CSRF check
finding: The plan applies requireSameOrigin to non-safe methods (POST/PUT/PATCH/DELETE). Verification of internal/dataentry/commands.go:254-256 shows handleCommandExec accepts BOTH GET and POST. A malicious site can issue `<img src="http://localhost:8080/api/command/{id}">` (a simple GET) and get RCE — the most damaging endpoint in the app, completely bypassing the proposed middleware. The same risk applies to any other endpoint that accepts GET for state-changing operations (need a sweep). The plan must either (a) make all mutating endpoints POST-only, or (b) apply requireSameOrigin to ALL methods on sensitive endpoints rather than method-based filtering.
severity: critical
resolution: 'Plan updated: (1) `handleCommandExec` is restricted to POST only as part of this ticket; (2) `requireSameOrigin` is applied to ALL methods on sensitive endpoints (`/api/command/`, `/api/open-file`, `/api/git/sync`, `/api/v1/_*`, `/api/entities`, `/api/relations`, `/api/v1/{plural}/*`), not just non-safe methods. Read-only GETs that don''t return sensitive content (e.g. static assets, SPA shell) keep the lighter Host-only check.'
status: addressed
---
