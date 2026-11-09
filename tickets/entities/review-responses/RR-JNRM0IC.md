---
id: RR-JNRM0IC
type: review-response
title: Peer credential bypasses the HostConfig secrets seam
finding: peerDeps read secrets.Global directly, so the desktop keychain HostConfig was ignored.
severity: significant
resolution: peerDeps reads the credential from lua.Host.Secrets("") like subscriptions do; the scheduler tests use a fake host.
status: addressed
---
