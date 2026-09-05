---
id: RR-RJ3QYT
type: review-response
title: NewLayered panicked instead of returning an error
finding: NewLayered validated its two required collaborators with a panic. CLAUDE.md requires a New* function with required collaborators to return error and validate up front; state.NewValidatedKV and kvstate.New both do. A panic also fails to catch the case that actually occurs in production — a non-nil interface holding a nil pointer — which sails past the nil check and fails at the first read anyway, exactly the deferred failure the rule prevents.
severity: critical
resolution: Changed to NewLayered(primary, secondary) (Loader, error) returning an error. Test rewritten to assert an error plus a nil Loader rather than a panic, and extended with a both-nil case. Callers are test-only so far; a mustLayered helper keeps the tests readable.
status: addressed
---
