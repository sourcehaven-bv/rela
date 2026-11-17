---
id: RR-VDKI41M
type: review-response
title: '[security] Schema reload drops the SQLite audit sink'
finding: '[security] On sqlite a reassembly built a new sink and closed the old one in CloseAssembly, so records written during and after a schema reload could be lost or go to a closed sink.'
severity: critical
resolution: The sink now lives on SharedBase and is carried by ForReassembly, so one sink serves the whole reassembly chain. It is closed in Services.Close only. Pinned by TestSQLiteAuditSurvivesReassembly.
status: addressed
---
