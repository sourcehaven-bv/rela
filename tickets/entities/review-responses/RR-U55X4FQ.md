---
id: RR-U55X4FQ
type: review-response
title: Capture tasks reach processes without an evaluator
finding: rela scheduler refuses only at startup; WatchConfig can add a capture task later. On postgres any process can take a queued capture job.
severity: significant
resolution: Capture job kind is registered only when an evaluator is present; a scheduler without one skips capture tasks at load and on reload and logs it; rela scheduler also refuses at startup.
status: addressed
---
