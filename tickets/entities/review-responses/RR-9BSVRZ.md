---
id: RR-9BSVRZ
type: review-response
title: Supersede window of 64 messages lets a retry overwrite a newer value in a busy channel
finding: 'internal/realtime/outputs.go:388,395: only the 64 messages after the notice are scanned. A retry may run up to OutputRetryWindow (30 min) later. If move B''s done set of the same property lies more than 64 messages after A''s notice, A''s retry does not see it and writes the stale value. Fix: page through the later messages up to the log end (notices are sparse, so filter on notice type), or record per channel the latest done generation per property on the head or session, and compare A''s Gen to it.'
severity: significant
resolution: supersededBy pages to the end of the log, so there is no window; TestOutputs_SupersedeBeyondFirstPage covers a busy channel.
status: addressed
---
