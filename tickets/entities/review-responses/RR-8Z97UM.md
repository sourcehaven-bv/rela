---
id: RR-8Z97UM
type: review-response
title: Per-path warn-once cache is global mutable state that leaks between tests
finding: |-
    The de-duplication fix required by RR-V1G22E introduces package-level mutable state (a sync.Map of already-warned paths). That creates two problems the plan must address before implementation.

    First, test isolation: the acceptance criteria include "a 0644 file logs exactly one warn" and "a 0600 file logs nothing". With a package-level cache these tests become order-dependent — a path warned in an earlier test stays suppressed. t.TempDir() gives each test a unique path which mostly avoids collisions, but relying on that is implicit and fragile.

    Second, the tests need t.Setenv for CREDENTIALS_DIRECTORY, which panics if the test calls t.Parallel(). internal/mail/mail_test.go uses t.Parallel() liberally, so the convention in neighbouring packages is not automatically safe here.
severity: minor
resolution: 'Added unexported resetPermissionWarnings(), called from the captureWarnings test helper both up-front and via t.Cleanup, so warning assertions cannot depend on test execution order. No t.Parallel() in any test using t.Setenv. Coverage concern resolved: internal/secrets is at 96.1% against the default floor of 50, with the new error branches (stat failure, non-regular credential, invalid credential YAML, empty relaDir) all covered.'
status: addressed
---

## Fix

Expose an unexported reset hook used only from tests (`func resetWarnCache()`),
called via `t.Cleanup` in every test that asserts on warning output. This is the
standard Go approach for package-level caches and keeps the cache unexported.

Do not add `t.Parallel()` to the new tests that call `t.Setenv`; the Go runtime
panics on that combination. Existing non-env tests in the package are
unaffected.

Coverage note: `internal/secrets` has **no override** in `.testcoverage.yml`, so
it sits at the default package floor of 50. The package is small, so the added
branches (stat error, windows skip, credentials-dir fallback) need tests or the
floor could be threatened — the error paths are the easiest ones to leave
uncovered.
