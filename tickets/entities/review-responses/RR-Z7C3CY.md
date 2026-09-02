---
id: RR-Z7C3CY
type: review-response
title: 'Multi-file staging: partial batch failure is unspecified (which files uploaded, what the user re-picks)'
finding: |-
    The plan handles `max > 1` for staging (stage up to max, all upload sequentially) and handles 'an upload fails' as a single condition. It does not specify partial-batch outcomes.

    With 3 staged files where #2 is rejected by the MIME scanner: does #3 still attempt? Does the error name WHICH file failed? After landing on the created entity, the user sees #1 attached and must work out that #2 and #3 are missing — and #3 was never actually attempted, though nothing tells them that.

    The plan's error text ('shows the server's problem+json detail inline') is per-request, so with N files the user could get N error toasts, or one that names no filename. Neither is specified.

    This is minor rather than significant because `max > 1` properties are the less common case and no data is destroyed — the source files still exist on the user's disk. But 'which of my files actually made it' is a question the current plan cannot answer.

    Fix: specify continue-on-error (attempt every file, collect failures) and an error message that names the failed filenames, e.g. 'Entity created. 2 of 3 files failed: b.pdf (too large), c.exe (type not allowed).' Add a test for the mixed-outcome batch.
severity: minor
resolution: 'Added an explicit Edge Case: continue-on-error, so every staged file is attempted; failures collected as {filename, error}; one message names them (''Entity created. 2 of 3 files failed: b.pdf (too large), c.exe (type not allowed).''). The rationale is recorded — aborting the loop would leave later files unattempted with nothing telling the user, and a bare per-request problem+json detail names no filename. Added a test-plan row for the mixed-outcome batch.'
status: addressed
---

## Finding

The plan treats upload failure as a single boolean. With `max > 1` it is not.

Given 3 staged files where #2 is rejected:

- **Continue or abort?** Unspecified. If the loop aborts, #3 is never attempted
and nothing tells the user that.
- **Which file failed?** The plan surfaces "the server's problem+json detail",
which describes the *rejection reason* but carries no filename — the endpoint is
per-property, and the client made N separate calls.
- **How many toasts?** N failures could mean N error toasts.

After navigation the user sees file #1 attached and must reverse-engineer what
happened to #2 and #3.

## Severity

Minor: `max > 1` is the less common configuration, and nothing is destroyed —
the originals are still on the user's disk. But "which of my files made it" is a
question the plan as written cannot answer, and the answer is cheap to specify
now versus discovering it in review.

## Suggested resolution

- Continue on error; attempt every staged file.
- Collect `{filename, error}` pairs.
- One message naming the failures:
`Entity created. 2 of 3 files failed: b.pdf (too large), c.exe (type not
allowed).`
- Test the mixed-outcome batch (some succeed, some fail).
