---
id: RR-B59JGQ
type: review-response
title: newCapturer mutable package var is a parallelism footgun / non-idiomatic seam
finding: 'go-architect (2nd pass): the injectable newCapturer was the only mutable-package-var test seam in the codebase (house idiom is consumer-side interfaces / per-instance seams). It coupled ''which tests can t.Parallel()'' to ''which tests stub the global'' — a latent race footgun. Recommended a nil-defaulting field on BuildCmd instead.'
severity: minor
resolution: Replaced the package var with a `newCapturer func() (docs.Capturer, error)` field on BuildCmd (nil → the build-tagged package NewCapturer, via a c.capturer() resolver). Tests assign the field per-command (okCapturer/noBrowser factories) and are all t.Parallel() again; -race clean. Removes the novel global and the parallelism hazard.
status: addressed
---
