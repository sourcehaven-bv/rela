---
id: RR-LMKYGDF
type: review-response
title: Schema skew aborts the run with prose
finding: A type missing on the remote or with different ends gives a 400 for the whole request. The user needs a clear message naming the type and how to narrow the scope.
severity: significant
resolution: A type the remote does not know gives a 400 whose detail names the type and now the range index; the error reaches the user sanitized. The CLI docs say to narrow --types to the shared types.
status: addressed
---
