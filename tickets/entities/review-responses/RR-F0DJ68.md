---
id: RR-F0DJ68
type: review-response
title: 'afterEach teardown was not exception-safe'
finding: 'If one wrapper''s unmount() threw, the remaining wrappers were never unmounted and the array was never cleared, leaking them into the next test. Latent rather than live, since each file mounts once per test.'
severity: minor
resolution: 'Fixed in all three files: splice the array first so a throw cannot leak it, then wrap each unmount in try/catch.'
status: addressed
---
