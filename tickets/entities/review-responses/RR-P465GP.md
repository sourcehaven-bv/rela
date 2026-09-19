---
id: RR-P465GP
type: review-response
title: Get conformance never asserts UpdatedAt, the field most likely to diverge
finding: The conformance suite had no coverage of Comment.UpdatedAt at all, and the new Get tests did not add any — despite it being the column most likely to diverge between the two read paths (optional, so it travels a NULL path; decoded separately per backend as *time.Time on postgres and *string on sqlite). The UTC-location assertion was written for CreatedAt and omitted for its sibling one line below.
severity: significant
resolution: 'Added an unedited-comment zero-value case and a full Get-vs-List comparison after an update to runGetFidelityTests. This surfaced a genuine pre-existing defect: filecomments and memcomments never stamp UpdatedAt while both database backends do, so the same edit yields a different record per build. Filed as TKT-JZY2PM; the suite pins only that Get and List agree, which is the part Get owns.'
status: addressed
---

## Finding

`grep -rn "UpdatedAt" internal/comments/commentstest/` returned nothing: the
conformance suite had no coverage of that column at all. The new `Get` tests
walked past it too — `returns the stored comment` asserted ID, Author, CreatedAt
(including `Location()`), Anchor, Body and Resolved but not UpdatedAt, and
`reflects an update` called `Update` (the only call that writes `updated_at`)
then asserted only Body and Resolved.

The hazard is specific. The column is optional, so it travels a NULL path the
others do not, and each backend decodes it separately: a `*time.Time` on
postgres, a `*string` parsed from text on sqlite. A `Get` returning it in the
server's local zone while `List` returned UTC is exactly the divergence
`RunGetTests`' own doc comment claims the suite exists to catch — and the UTC
assertion was written for `CreatedAt` and not for its sibling one line below.

## Resolution

Added to `runGetFidelityTests`:

- `an unedited comment has a zero UpdatedAt` — pins the NULL path, so `omitzero`
keeps meaning "never edited" to an API client.
- `reflects an update` now compares the full `Get` result against the `List`
element (`require.Equal(t, listed[0], got)`), and asserts UTC on any stamped
value.

This surfaced a real pre-existing defect rather than merely closing a gap: the
assertion passed on both database backends and FAILED on filecomments and
memcomments, which never set `UpdatedAt` on update. So the same edit produces a
different record depending on the compiled-in backend, and `updated_at` is
absent from every API response on the default build.

That is out of scope for a read-amplification ticket, so it is filed as
TKT-JZY2PM. The suite deliberately pins only what `Get` owns — that its two read
paths agree with each other — rather than asserting a contract the backends do
not yet share.
