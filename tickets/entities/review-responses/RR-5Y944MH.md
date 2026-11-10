---
id: RR-5Y944MH
type: review-response
title: The header GraphQuery scans every run header on fs and mem
finding: The proposed status-filtered GraphQuery on _flow-run reads every header ever written on the naive path, on every governed GET and PATCH.
severity: significant
resolution: 'Addressed in plan: replaced by the run-store lookup (finding on header lag); no header query and no new storetest.'
status: addressed
---
