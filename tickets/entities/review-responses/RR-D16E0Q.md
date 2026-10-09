---
id: RR-D16E0Q
type: review-response
title: No count/list parity test under a world
finding: TestCountPushdown_EqualsListPushdown used AllFaces only and memstore only
severity: significant
resolution: Added draft-first world rows for every principal including the face-restricted grant. The check is a helper run on memstore and under -tags sqlite on sqlitestore. No local postgres; the pg count SQL was checked by reading buildEntityCountSQL.
status: addressed
---
