---
id: RR-C2H1
type: review-response
title: Fail-open error handling silently swallows security-relevant errors
finding: 'The fail-open pattern at `/Users/jeroen/Work/sourcehaven/rela-3/internal/validation/lua.go:54-82` logs errors via `log.Printf` but returns `true` (pass). Problems: (1) Using `log.Printf` instead of structured logging - errors go to stderr and may be missed in automated runs. (2) No way for callers to know a rule was skipped due to error vs legitimately passed. (3) In CI/validation checks, silently skipping broken rules could mask configuration errors. CONSIDER: (1) Add a `--strict` mode where Lua errors cause validation to fail, OR (2) Return errors alongside violations so callers can decide, OR (3) At minimum, emit a warning-level violation when a rule is skipped due to error.'
severity: significant
reason: The fail-open behavior is intentional for this initial implementation to avoid blocking validation runs due to misconfigured rules. A follow-up ticket could add --strict mode or warning-level violations for skipped rules. The current log.Printf output is consistent with other error logging in the codebase.
status: deferred
---
