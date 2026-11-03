---
id: RR-7UN6X2
type: review-response
title: filechannels crash-window doc understates what a timer crash now loses
finding: 'internal/channels/filechannels/file.go:62-66 still says a crash after meta.yaml leaves the session moved ''with no notice and no audit record''. SetSession (ops.go:1199-1207) now puts the outputs mark on that notice. The same crash therefore also drops the timer''s run_as graph writes and its on_final close: the channel stays open in a final state, and nothing retries because the pending ref is dropped by dropDangling. Fix: document that consequence in the package doc and in the guide''s Timers/Outputs section (or note it in the plan as accepted). sqlchannels is atomic and unaffected.'
severity: minor
resolution: filechannels keeps a timer/trigger move's log records in meta.yaml; Append, SetSession and StalePending write them back after a crash, so the mark, outputs and on_final close survive; 3 tests; doc updated.
status: addressed
---
