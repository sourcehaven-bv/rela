---
id: RR-I6GF36
type: review-response
title: rela db migrate exited non-zero for a successful no-op; CI lacked negative tag assertions
finding: Two issues. (1) runDBMigrate and runDBReconcile returned errors for what are genuinely no-ops on this build, so `rela db migrate && start` breaks and a sqlite build in a pipeline running the documented `rela db reconcile --dry-run` CI drift gate fails unconditionally. The postgres equivalent prints 'Database is up to date' and returns nil. (2) CI compiled each backend tag in isolation but never asserted that CONFLICTING combinations still fail -- the valuable negative. If someone later 'fixed' a redeclaration by narrowing a tag, mutual exclusion would degrade into one recipe arbitrarily winning and CI would stay green.
severity: significant
resolution: All three db commands return nil and print what is true, reporting the real schema version. Added CI assertions that sqlite+postgres, sqlite+memorybackend and postgres+memorybackend all fail to compile, with a comment explaining that the redeclaration IS the mutual-exclusion mechanism and is only load-bearing if pinned. Verified all three are rejected locally.
status: addressed
---
