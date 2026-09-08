---
id: RR-HRC7JC
type: review-response
title: 'PR-B: family-invariant probe was check-then-act under READ COMMITTED - headless states reachable'
finding: The CreateEntity family probe was a plain SELECT inside the tx; under READ COMMITTED a concurrent family delete could commit between probe and insert, materializing exactly the headless state the invariant forbids (reviewer demonstrated the interleaving live at SQL level). fs/mem hold a process mutex across probe-and-write; pg — the multi-process backend — was the one without real serialization, and the comment claimed safety.
severity: critical
resolution: 'The probe now takes FOR SHARE on the default row, blocking a concurrent family delete until the insert commits; the comment states the actual mechanism. (The update-path probe needs no lock: a racing delete makes the subsequent UPDATE affect zero rows → ErrNotFound, not corruption.) The reviewer''s leverage note — making headless states unrepresentable via a self-referential FK with ON DELETE CASCADE — is recorded for the architect as the shape to converge to.'
status: addressed
---
