---
id: RR-5F3KE
type: review-response
title: No test documents the attachments-are-not-watched invariant
finding: Attachments tree is not watched today; if future code adds it, OpenForWrite's lack of observer-fire would cause duplicate events. Add a defensive test.
severity: significant
reason: Speculative defense against a future change. The comment in attachment.go already explains why streaming bypasses the observer; if someone adds attachments to the watcher they'll need to confront the observer question anyway. Not worth locking in a test that asserts a non-feature.
status: wont-fix
---
