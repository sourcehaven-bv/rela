---
id: RR-K072YL
type: review-response
title: Resume completeness assumes a lossless store feed
finding: store.Watcher drops events on a full buffer without signalling; docs promised resume loses nothing.
severity: minor
resolution: runSSELoop doc and api-reference.md now say resume covers every change the store delivered; a change the store dropped is missed by live streams and resume alike. A drop signal in store.Subscribe is out of scope.
status: addressed
---
