---
id: RR-MSZ101
type: review-response
title: Resolve gate runs on the unprovisioned request
finding: The git:sync gate runs before withProvision while the write check runs after. This matches handleGitSync but could be changed in one place only.
severity: minor
resolution: A comment at the gate says the ordering is deliberate and matches handleGitSync.
status: addressed
---
