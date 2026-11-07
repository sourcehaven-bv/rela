---
id: RR-GXRLL96
type: review-response
title: 'Code: differential test can pass vacuously'
finding: diffElementRange treats both-errored as agreement and nothing counted whether the 300 cases exercised anything; sqlite fallback cases compare the reference with itself.
severity: significant
resolution: RunElementRangeDifferential now requires a quarter of the cases to admit rows, a tenth to mix folded and pending rows, and (via sqlitestore.RendersRangeNatively) half to take the SQL path.
status: addressed
---
