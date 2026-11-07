---
id: RR-OA5U8FX
type: review-response
title: who-can read reports no readers for a flow run
finding: AccessRoutes credited only roles that grant read on _flow-run, so every reader of the subject was dropped.
severity: significant
resolution: flowRunReadRoutes reports the routes by which the reader reads each subject. Pinned by TestFlowRunAccessRoutes.
status: addressed
---
