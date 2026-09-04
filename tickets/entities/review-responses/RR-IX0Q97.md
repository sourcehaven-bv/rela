---
id: RR-IX0Q97
type: review-response
title: Extended-length UNC row asserted an unverified VolumeName result
finding: |-
    While adding clone_windows_test.go for RR-S2X70O, both test files gained a row
    claiming `\\?\UNC\server\share` is a volume root. That claim was never checked
    against the stdlib -- it was pattern-matched from the bare-UNC case.

    It is wrong. volumeNameLen's `\\.\UNC` special case keys on the `.` form
    (pathHasPrefixFold(path, `\\.\UNC`)), so the `?` form falls through to the
    generic `\\?` branch, which cuts at the first separator after the 4-byte
    prefix and yields a volume of `\\?\UNC`. The share would therefore be a path
    UNDER a volume, not a root, and the row in clone_windows_test.go -- the file
    that derives values from the real filepath -- would have FAILED on Windows.

    The irony is the point: this is the exact defect RR-S2X70O had just been raised
    about, committed in the fix for it. The Linux table cannot catch it (it supplies
    the volume itself, so it is self-consistent whatever it says) and the Windows
    file that could catch it does not run on CI. Nothing in the pipeline would have
    reported this.
severity: significant
resolution: |-
    The `\\?\UNC` row was REMOVED from both files rather than corrected, and each
    file now carries a comment saying why it is absent: the behaviour is not pinned
    by the stdlib's own volumenametests either, so asserting either answer would be
    another unverified guess.

    Removing beats guessing here. A wrong assertion in the Windows file is a test
    that fails on the only platform it runs on; a wrong assertion in the Linux
    table is a green test that means nothing. Neither is better than silence about
    a form no caller uses.

    `\\?\C:` (bare and with trailing separator) is kept, because the stdlib's
    volumenametests DOES pin the generic branch: `\\?\x` maps to `\\?\x`. Verified
    claims stay, guessed ones go.
status: addressed
---

## Resolution

Removed the `\\?\UNC\server\share` row from `clone_test.go` and
`clone_windows_test.go`, replacing it in each with a comment recording why the
case is deliberately unasserted.

Kept `\\?\C:` in both, since the stdlib's own `volumenametests` pins the generic
`\\?` branch (`\\?\x` → `\\?\x`), so that one is verified rather than assumed.

Mutation re-run after the correction: disabling the bare-volume clause still
reddens `unc share root bare volume` and `extended drive volume`, so the
narrowed table has not lost its grip on the fix.
