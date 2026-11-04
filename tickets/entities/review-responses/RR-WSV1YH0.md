---
id: RR-WSV1YH0
type: review-response
title: Stale HEAD comment and no-op poll in forms.spec
finding: The comment mentions a HEAD request that is never sent and the poll passes on its first sample.
severity: nit
reason: Pre-existing code outside this flake; submit already waits for the URL. Left for a separate cleanup to keep the diff to the flakes.
status: wont-fix
---
