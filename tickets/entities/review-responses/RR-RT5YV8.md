---
id: RR-RT5YV8
type: review-response
title: arch-lint visibility.mayDependOn lists unused 'store' — over-broad allowance
finding: 'The production package never imports internal/store (store.Store satisfies the one-method EntityGetter structurally); only the excluded visibilitytest does. An unused allowance weakens the boundary: a future direct store import would pass arch-lint unnoticed. Remove store from the block.'
severity: minor
resolution: Removed store from visibility.mayDependOn; block comment now states the package deliberately takes store.Store structurally through the one-method EntityGetter. arch-lint green.
status: addressed
---
