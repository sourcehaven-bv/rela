---
id: RR-8Q8SBIS
type: review-response
title: 'Code: Pending has no guard against the wire'
finding: RangeSum.Pending and RangeEntry.Pending are documented as server-internal but an encoded value would carry them.
severity: minor
resolution: Both fields carry json:"-".
status: addressed
---
