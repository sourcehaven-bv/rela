---
id: RR-HV99QA
type: review-response
title: Download cap checked after buffering
finding: maxDownloadBytes was checked after httptest.NewRecorder held the whole body, so it bounded nothing.
severity: significant
resolution: cappedRecorder refuses writes past the limit while the handler writes. Tested.
status: addressed
---
