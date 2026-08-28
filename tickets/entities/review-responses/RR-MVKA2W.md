---
id: RR-MVKA2W
type: review-response
title: CredentialName produced "rela-secrets-.." for an upward relative path
finding: |-
    CredentialName("../.rela") returned "rela-secrets-.." — filepath.Base(filepath.Dir(filepath.Clean("../.rela"))) is "..", which was not in the guard alongside "." and the separator. The result is a credential name that names no project and that an operator would never deliberately declare, so any file matching it in $CREDENTIALS_DIRECTORY would be picked up on the strength of a path artifact rather than an operator decision.

    Found by running CredentialName over a table of degenerate inputs rather than by reading it; the original guard covered the two cases I had thought of and missed the third.
severity: minor
resolution: 'Added ".." to the guard so the credentials source is disabled (returns "") rather than deriving a nonsense name. Covered by two new TestCredentialName subtests: "upward relative path names no project" and "filesystem root".'
status: addressed
---
