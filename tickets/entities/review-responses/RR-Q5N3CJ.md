---
id: RR-Q5N3CJ
type: review-response
title: 'A root BaseDir made containment a no-op that looked like it passed'
finding: |-
    containedPath("/", "/etc/passwd") returned successfully: filepath.Rel gives
    "etc/passwd", no "..", so the check passes. Correct per the containment
    contract, and useless -- a root base contains every path.

    Same silent-no-op shape as the empty base this ticket fixes, reached by a
    different route: a config default resolving to "/", or a caller passing it
    deliberately. Worse than the empty case in one respect, since the check appears
    to have run and verified something.
severity: significant
resolution: |-
    containedPath now rejects a base that cleans to the filesystem root. There is no
    legitimate reason to scope a clone to the whole filesystem, so refusing costs
    nothing and closes the last way to get containment-shaped code that checks
    nothing.

    Pinned by TestContainedPath_RejectsRootBase, mutation-verified: removing the
    guard reddens it alone.
status: addressed
---

## Resolution

containedPath now rejects a base that cleans to the filesystem root. There is no
legitimate reason to scope a clone to the whole filesystem, so refusing costs
nothing and closes the last way to get containment-shaped code that checks
nothing.

Pinned by TestContainedPath_RejectsRootBase, mutation-verified: removing the
guard reddens it alone.
