---
id: RR-0DTTMS
type: review-response
title: 'Bare UNC volume root missed: Clean does not append a trailing separator'
finding: |-
    isFilesystemRoot missed the bare UNC volume `\\server\share`, which is the form
    filepath.Clean actually returns for that input.

    The predicate compared cleanedPath against volumeName+separator only. For
    `\\server\share` the ENTIRE string is the volume name (uncLen finds no second
    separator after the `\\` prefix and returns len(path)), so Clean hits its
    `path == ""` branch and returns the input unchanged -- with no trailing
    separator. The comparison was therefore `\\server\share` vs `\\server\share\`,
    which is false.

    That is a live fail-open, not a theoretical gap: filepath.Rel deliberately
    treats a bare `\\host\share` base as absolute (its own comment says so), so
    containedPath accepted the share root and then reported every path on the share
    as contained -- exactly the condition this ticket exists to close, on the exact
    platform it is about.

    Worse, the test table ASSERTED the wrong form. Its `unc share root` row supplied
    `\\server\share\`, a string Windows never produces for that input, so the row
    passed green over a case that could not occur while the case that does occur
    went unguarded. Same failure mode as the defect: something shaped like a check
    that verifies nothing.
severity: critical
resolution: |-
    isFilesystemRoot gained a second clause matching the bare-volume form, guarded
    on a non-empty volume so it is Windows-only. Verified against the stdlib
    (uncLen, Clean's empty-remainder branch, Rel's own comment about
    `\\host\share`) rather than assumed. The wrong table row was corrected and the
    real form added beside it, plus the extended-length spellings Clean also
    returns verbatim.

    Mutation-verified: disabling the clause reddens exactly the three bare-volume
    rows; dropping the non-empty-volume guard reddens the empty-path row.
status: addressed
---

## Resolution

`isFilesystemRoot` gained a second clause for the bare-volume form:

```go
if volumeName != "" && cleanedPath == volumeName {
    return true
}
return cleanedPath == volumeName+string(separator)
```

The `volumeName != ""` guard confines the new clause to Windows — on Unix
`VolumeName` is always `""`, and an empty cleaned path must not be called a
root.

Verified against the stdlib rather than assumed, since assuming is what caused
the defect. `uncLen` (`internal/filepathlite/path_windows.go:279`) scans for a
SECOND separator after the `\\` prefix and returns `len(path)` when it finds
none, so `volumeNameLen("\\\\server\\share")` is the whole string. `Clean` then
takes its `path == ""` branch and returns the input verbatim — no trailing
separator appended. And `filepath.Rel` (`path/filepath/path.go:197-200`)
explicitly rewrites an empty post-volume base to a separator "for any targetpath
matching `\\host\share`", which is the stdlib confirming in its own comment that
it treats the bare share as an absolute base. So every path on the share was
reported contained.

Also covered in the same clause: `\\?\UNC\server\share` and `\\?\C:`, which
Clean likewise returns verbatim.

Mutation-verified: disabling the new clause reddens exactly the three
bare-volume rows (`unc share root bare volume`, `extended unc share root`,
`extended drive volume`) and nothing else. Dropping the `volumeName != ""` guard
instead reddens `empty path no volume`, which is what pins that the fix narrows
only to genuine roots.
