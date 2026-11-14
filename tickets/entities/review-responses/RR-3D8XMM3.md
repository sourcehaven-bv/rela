---
id: RR-3D8XMM3
type: review-response
title: mail.yaml is read from disk without the fixture symlink rule
finding: testHost.File reads mail.yaml through hostconfig.Dir, not rootfs, so it has no symlink refusal.
severity: minor
reason: This is how the case read mail.yaml before this change and how production reads it; mail.yaml holds no secret values, and fixture rules apply to files a test names, not to project config.
status: wont-fix
---
