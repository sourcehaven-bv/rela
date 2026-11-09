---
id: RR-WIXLJP4
type: review-response
title: 'Design: key reload blocking and diagnostics'
finding: Reload under a mutex could block requests on file I/O; a malformed entry locks out all peers silently; a systemd credential does not change at runtime.
severity: minor
resolution: The source loads once at construction; later reloads are single-flight with TryLock and other requests use the published snapshot. A load error publishes an empty keyring and logs the entry name once per attempt. The guide states that a systemd credential needs a restart.
status: addressed
---
