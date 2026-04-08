---
id: RR-REZH5
type: review-response
title: buildRenameResult ran from pre-tx snapshot
finding: The result returned by Rename was built from the snapshot loaded BEFORE WithTx, while the actual operation ran against the in-tx snapshot. If those differed, the result would describe a different rename than what was committed.
severity: significant
resolution: Fixed in this PR. For the non-dry-run path, buildRenameResult is now called inside the WithTx closure using tx.base.meta and the in-tx incoming/outgoing lists. The dry-run path keeps the pre-tx snapshot because it doesn't promise consistency.
status: addressed
---
