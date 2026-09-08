---
id: RR-S2X70O
type: review-response
title: Hand-transcribed Windows values can be wrong and still pass green
finding: |-
    The parameterised-predicate technique gets the Windows branch EXECUTING on Linux
    CI, which was the goal, but it does not make the test true. Every input in
    TestIsFilesystemRoot is a hand transcription of what filepath.Clean and
    filepath.VolumeName are BELIEVED to return on Windows, and nothing in the
    package checks that belief. A wrong transcription produces a green test over a
    string Windows never emits.

    This is not hypothetical: it is exactly how the bare-UNC defect (RR-0DTTMS)
    survived the first implementation. The table asserted `\\server\share\`, Clean
    returns `\\server\share`, and the row passed while the real form went unguarded.

    The ticket's own framing invited this. It treated "the branch executes on Linux
    CI" as the property to achieve, and that property was achieved by a test whose
    inputs were fiction. Executing a branch with inputs the platform cannot produce
    is coverage without verification.
severity: significant
resolution: |-
    Added internal/git/clone_windows_test.go (//go:build windows), which never
    spells out a cleaned form: it Cleans the input and feeds the result straight
    into the predicate, so it cannot inherit a mistaken belief about Clean. It
    covers the drive root, both UNC spellings, the extended-length forms,
    drive-relative C:foo, and the end-to-end containedPath refusal.

    It does NOT run on Linux CI, so it does not replace the table -- the two are
    deliberately complementary, and both test files say so. The table is what runs
    on every PR; the Windows file is what stops the table drifting from reality and
    is runnable by a maintainer on Windows to check the claim directly. GOOS=windows
    go vet keeps it compiling on CI even though it does not execute there.
status: addressed
---

## Resolution

Added `internal/git/clone_windows_test.go` behind `//go:build windows`. It
derives the `(volume, cleaned, separator)` triple from the host `filepath`
instead of from memory, so it cannot repeat a transcription error. Both test
files now document the split: the Linux table is what executes on CI, the
Windows file is what keeps the table honest.

`GOOS=windows go vet ./internal/git/` passes, so the file stays compiling on CI
even though it does not run there.
