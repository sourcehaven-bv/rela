---
id: RR-RNJV8D
type: review-response
title: Missing store capability fails per poll rather than at start
finding: A workspace without Store() passes config load and fails every poll; Deps.validate does not check ingest deps.
severity: minor
reason: Every real workspace (appbuild.Services) provides Store; the per-poll error names the missing capability and fails the run visibly. ingest.Run validates its deps on the first write.
status: wont-fix
---
