---
id: RR-DF9B2OM
type: review-response
title: A missing runner keeps retrying
finding: Before the data-entry app installs the runner the job is retryable.
severity: minor
reason: The runner is installed during startup, so the window is short; RetryBounded ends the job after its attempts.
status: wont-fix
---
