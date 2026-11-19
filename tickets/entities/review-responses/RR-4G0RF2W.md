---
id: RR-4G0RF2W
type: review-response
title: expectSectionAbsent passes on an unloaded page
finding: toHaveCount(0) passes at once before the view renders; the helper relied on a prior call.
severity: nit
resolution: The helper now waits for a view section to be visible before asserting absence.
status: addressed
---
