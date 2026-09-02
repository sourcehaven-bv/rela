---
id: RR-QQQ2JR
type: review-response
title: 'Missing tests for the dangerous paths: double-submit, unmount mid-upload, wizard-hidden, duplicate key'
finding: |-
    The suites cover the happy path and failure messaging well, but not the paths that actually break:

    - double-submit (RR-HJLLUF) — the reproduced defect
    - navigate-away mid-upload: router.push fires AFTER unmount. Every other async path in DynamicForm guards this (stagedUnmounted at line 674, the RR-2PZB guard); uploadStagedFiles has no equivalent. Benign today (push on a stale router is a no-op) but it is the only async block that breaks the file's established discipline.
    - wizard revealed-then-hidden file property (RR-OI6P51)
    - duplicate key / same file staged twice (RR-C6CXU1)

    e2e has no failure-path test at all. The single highest-value case — create succeeds, upload 413s, user sees the error and lands on the entity — is exactly the accepted tradeoff the design documents, and it is untested end-to-end even though the server has a real configurable size limit that makes it easy to drive.
severity: significant
resolution: 'All four gaps now covered. Unit: ''ignores a second submit while one is in flight'' and ''a second submit after a failure does not create a second entity'' (double-submit); ''does not navigate when the component unmounted mid-upload'' (plus a `stagedUnmounted` guard before router.push, matching the RR-2PZB discipline every other async path here follows); ''does not upload a file staged under a then-hidden wizard branch'' (wizard); index-keyed removal covered by the widget suite. E2E: ''an oversize file fails loudly after the entity is created'' drives a REAL 413 (fixture sets max_attachment_bytes: 1024) and asserts the entity exists, the user lands on it, the message names the file and says where to re-attach, and no property was stamped.'
status: addressed
---

## Finding

The tests pin the happy path and the failure *message*, but not the paths where
this feature actually breaks.

Missing:

1. **Double-submit** (RR-HJLLUF) — the reproduced defect.
2. **Unmount mid-upload.** `router.push` fires after unmount. Every other async
path in `DynamicForm` guards this — `stagedUnmounted` (line 674), the RR-2PZB
guard — and `uploadStagedFiles` has no equivalent. Benign today because `push`
on a stale router is a no-op, but it is the only async block in the file that
ignores the established discipline, which is how it stops being benign later.
3. **Wizard revealed-then-hidden file property** (RR-OI6P51).
4. **Duplicate key / same file staged twice** (RR-C6CXU1).

## e2e gap

`attachments-create.spec.ts` has no failure-path test. The highest-value case —
create succeeds, upload 413s, user is told and lands on the entity — is the
accepted tradeoff the whole design rests on, and it is unverified end to end.
`max_attachment_bytes` is configurable, so a fixture can drive a real 413.
