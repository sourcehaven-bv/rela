---
id: RR-BRNVX9
type: review-response
title: Missed-edit guards (xmin/content guard and trigger columns) were untested
finding: Removing the xmin write-back guard, type from the entity trigger, or from_id from the relation trigger left the whole pgstore suite green; sqlite had the same gap.
severity: significant
resolution: Added TestSweepWriteBackSkipsARowWrittenAfterTheRead on both backends (a beforeCapture test hook writes the row between the candidate query and the write-back), plus SweepBacklog cases EntityType and RenameWithoutRenameVersion. Each fails when its guard or trigger column is removed (checked by mutation).
status: addressed
---
