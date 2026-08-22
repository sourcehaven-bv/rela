---
id: RR-5NIR95
type: review-response
title: Warn-test hijacks process-global slog with no parallel-safety guard
finding: 'appbuild_membership_warn_test.go''s captureWarnings calls slog.SetDefault — a process-global mutation. Safe today only because no other test in the package parallelizes around a service build, but nothing encoded that: the new tests lacked t.Parallel() with no comment while the sibling acl test uses it, so a future reflexive t.Parallel() would cause a logger data race AND cross-test log bleed that could turn the QuietWhenSafe negative assertions into vacuous passes — a false pass on a security-warning test.'
severity: significant
resolution: captureWarnings now carries a godoc explicitly forbidding t.Parallel() in callers and explaining both failure modes (race + log bleed => vacuous quiet-case passes). Verified with go test -race ./internal/appbuild/.
status: addressed
---
