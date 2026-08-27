---
id: RR-VHCGRL
type: review-response
title: UnixNano comparison overflows past year 2262
finding: compareValues compared temporal values via UnixNano() which overflows int64 outside roughly 1678-2262. A date of 2300-01-01 wraps to a large negative value and sorts before every other date. Introduced by this change; the original code used Unix() seconds which has no such limit.
severity: minor
resolution: Use time.Compare which is defined for the whole time.Time range. Three regression cases added and verified to fail against the UnixNano version.
status: addressed
---

## Finding

`compareValues` was changed to compare temporal values on `UnixNano()`.
Nanoseconds since the epoch overflow an int64 outside roughly **1678–2262**, so
a date beyond that range wraps negative and sorts *before* every other date.

Demonstrated: `2300-01-01` yields `UnixNano() = -8032952073709551616`.

The practical blast radius is small — a project would need dates past 2262 — but
the failure mode is bad: a far-future entity silently sorts to the wrong end of
a filtered list, or drops out of a calendar window, with no error. It is also a
defect this change *introduced*: the original code compared on `Unix()`
(seconds), which has no such limit at any plausible date.

## Resolution

Compare with `lt.Compare(rt)` instead, which is defined for the whole
`time.Time` range and does not go through an integer epoch at all.

Added three cases to `TestCompareValues_Datetime` covering a far-future date,
two far-future dates against each other, and a pre-epoch date. Verified they
FAIL against the `UnixNano` version.

Found by self-review of the widening's blast radius before the reviewer reported
it.
