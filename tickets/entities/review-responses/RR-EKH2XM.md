---
id: RR-EKH2XM
type: review-response
title: Body told error-kind readers to fetch crash files that do not exist
finding: 'The body branched the kind explanation correctly ("error means the target failed for an infrastructure reason... not necessarily a new crashing input") and then unconditionally instructed the reader to copy `testdata/fuzz/<Target>/` files from the artifact. For an `error` — a build failure or seed-corpus regression — no corpus file is written, and the artifact step is `if-no-files-found: ignore`, so the artifact may hold only the summary. The issue explained the finding was not necessarily a crash and then sent the reader after the crash.'
severity: significant
resolution: 'The reproduction paragraph moved inside the `*fuzz-crash*` arm of the existing `case`. The `error` arm now says there may be no corpus file and points at the run log for that target. Both bodies rendered and checked: the fuzz-crash body keeps the concrete repro command, the error body carries no testdata reference.'
status: addressed
---

Found by cranky-code-reviewer on the TKT-8LZGME diff.

Rendered `error` body after the fix:

```
| Kind | `error` |

Run: http://run/9

`error` means the target failed for an infrastructure reason,
e.g. a seed-corpus regression or a build failure — NOT necessarily a
new crashing input, so there may be no corpus file to reproduce from.
Start with the run log for this target.
```

The `fuzz-crash` body still carries the filled-in repro (`go test
-run='^FuzzGenerateShortID$' ./internal/entity`).
