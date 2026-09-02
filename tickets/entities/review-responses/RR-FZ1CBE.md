---
id: RR-FZ1CBE
type: review-response
title: 'Unsynchronized bytes.Buffer in the slog capture helper'
finding: |-
    `captureWarnings` returned a bare `*bytes.Buffer`, which is not safe for
    concurrent use. `newTestAppV1` builds a real app whose background goroutines can
    log after the constructor returns, so a write can race the assertion's read.

    The repo already documented this exact hazard and its fix:
    `internal/appbuild/appbuild_membership_warn_test.go:19-27` wraps the buffer in a
    `lockedWriter` and states callers must not use `t.Parallel()`, because
    slog.SetDefault mutates process-global state. There are 20 `t.Parallel()` calls
    elsewhere in this package; today the tests are safe by scheduling accident (Go
    runs parallel tests after serial ones), not by construction.
severity: significant
resolution: |-
    Adopted the `lockedWriter` pattern from the existing precedent, with the same
    "callers must NOT use t.Parallel()" note and the reason for it.

    Also fixed the empty subtest name in the modes table (`mode=` renders as an
    awkward `-run` target; now `mode=<empty>`), and replaced the `TKT-*` placeholder
    that shipped in two comments with the real id, `TKT-M60ZF5`.
status: addressed
---

## Resolution

Adopted the `lockedWriter` pattern from the existing precedent, with the same
"callers must NOT use t.Parallel()" note and the reason for it.

Also fixed the empty subtest name in the modes table (`mode=` renders as an
awkward `-run` target; now `mode=<empty>`), and replaced the `TKT-*` placeholder
that shipped in two comments with the real id, `TKT-M60ZF5`.
