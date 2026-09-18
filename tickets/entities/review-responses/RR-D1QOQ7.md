---
id: RR-D1QOQ7
type: review-response
title: Non-ISO date formats and mixed date/datetime diverge across the pushdown boundary
finding: 'Design review C3, which matches a defect found independently during planning. queryplan.StringShaped (queryplan.go:119) admits date/datetime by switching on pd.Type and never inspecting pd.Format (metamodel/types.go:762, a Go layout string). filter''s compareDates parses via ParseDateValue and honours the format. Measured: format "02/01/2006" gives Go [31/12/2025 05/01/2026 10/02/2026] (chronological) vs byte-wise [05/01/2026 10/02/2026 31/12/2025]. Latent today because applyV1Sorting is byte-wise so both paths agree while both are chronologically wrong; delegation activates it. AC2 will not catch it because the differential fixture uses ISO dates via metamodelProp("date") with no format. Two further cases the review raised: (a) mixed date/datetime columns are compared as instants in Go (comparePropValues, RR-5R3QFJ) but byte-wise in SQL, and timezone suffixes break the coincidental agreement since "Z" > "+" byte-wise while representing an earlier instant for a positive offset; (b) unparseable date values fall back to compareStrings, i.e. natsort, compounding RR-C4QYTO. Fix: StringShaped must decline a date/datetime carrying a non-default Format, and the mixed date/datetime decision must be recorded explicitly. ISO dates and same-day date/datetime pairs were probed and do agree, so the narrowing is limited.'
severity: significant
status: open
---
