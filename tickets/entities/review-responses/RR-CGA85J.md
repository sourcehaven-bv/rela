---
id: RR-CGA85J
type: review-response
title: Delete on a lowered-max property drops other entries
finding: A single-file property whose max was lowered can still hold several entries. Deleting one wrote back only the first remaining entry as a scalar, dropping the other references without deleting their bytes.
severity: significant
resolution: stampValue writes a list whenever more than one entry remains. TestFaced_DeleteOnLoweredMaxKeepsOtherFiles, mutation-checked.
status: addressed
---
