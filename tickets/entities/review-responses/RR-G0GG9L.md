---
id: RR-G0GG9L
type: review-response
title: edgeSourceType fallback branch untested
finding: Every fixture edge had a live named tail face, so the anyFaceOf fallback (tail face gone) and the fail-closed case (source with no rows) were never exercised.
severity: significant
resolution: TestDelete_CascadeSourceFallback covers a tail face that is gone while the family exists (allowed) and a source with no rows (denied, nothing deleted) on memstore, fsstore and sqlite.
status: addressed
---
