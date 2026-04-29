---
id: RR-7IVPA
type: review-response
title: Cache Intl.DateTimeFormat instance for hot-path use in lists
finding: toLocaleDateString re-parses options every call. For 500-row lists with 2 date columns that's 1000 parses per render. Instantiate new Intl.DateTimeFormat(undefined, DATE_FORMAT_OPTIONS) once at module load and call .format(date).
severity: minor
reason: Premature optimization. The reviewer cited 1000 calls per render for a 500-row list with 2 date columns; modern Intl.DateTimeFormat call cost is sub-microsecond and lists are paginated. No measurement showed an actual hot path. Defer until a real perf complaint surfaces; the centralization (DATE_FORMAT_OPTIONS) is already in place so the swap is one line if needed.
status: deferred
---
