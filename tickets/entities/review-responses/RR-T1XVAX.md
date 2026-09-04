---
id: RR-T1XVAX
type: review-response
title: TestContainedPath_UsesIsFilesystemRoot proved nothing the existing test did not
finding: |-
    The new wiring test walked filepath.Dir up from t.TempDir() to find the root,
    then asserted containedPath rejects it. On Linux that root is always "/", which
    is precisely the case the PRE-EXISTING TestContainedPath_RejectsRootBase already
    covers -- and the case where the old check (absBase == string(Separator)) and
    the new one agree exactly.

    So the test passes identically against the unfixed code. It reads as a
    guarantee that containedPath consults the new predicate, while being incapable
    of distinguishing the new predicate from the old one. That is the same
    check-shaped-but-empty pattern this ticket is fixing, reintroduced in the tests
    that fix it.

    Its extra machinery (the Dir walk, the setup self-assertion) also implied it was
    reaching somewhere the simpler test could not, which was not true on the
    platform CI runs.
severity: minor
resolution: |-
    Removed, and replaced with a comment at the same location saying why no such
    Linux test exists -- otherwise the obvious next move is to write it again.

    The wiring is covered where it can actually be distinguished:
    TestContainedPath_RejectsRootBase for "/" (unchanged, and mutation-verified: a
    disabled call site reddens it), and TestContainedPath_RejectsWindowsRootBases in
    clone_windows_test.go for the roots that separate old behaviour from new.
status: addressed
---

## Resolution

Removed the test. A comment now sits where it was, recording that any Linux test
of this wiring can only reach the predicate through `"/"` — where the old and
new checks agree — so it would be evidence of nothing while looking like a
guarantee.

Coverage of the wiring lives in two places that can tell the implementations
apart: `TestContainedPath_RejectsRootBase` for the Unix root (mutation-verified —
disabling the call site reddens it), and `TestContainedPath_RejectsWindowsRootBases`
in the build-tagged Windows file for the drive and share roots.
