---
id: RR-TVL38I
type: review-response
title: captureWarnings read unsynchronized against post-New background logging
finding: 'captureWarnings returned buf.String bound to a bytes.Buffer the slog handler could still be writing to: appbuild.New starts background goroutines (store watcher; postgres listener/sweep) that may log after New returns, and bytes.Buffer is not safe for concurrent use. -race was clean only by timing accident — a classic flaky-in-CI-only shape.'
severity: significant
resolution: Writes now go through a mutex-guarded lockedWriter and the returned accessor reads under the same mutex (internal/appbuild/appbuild_membership_warn_test.go). Verified with go test -race ./internal/appbuild/.
status: addressed
---
