---
id: RR-UUDJGE
type: review-response
title: Symmetric self-inverse ownership unpinned; minor test hardening
finding: A symmetric self-inverse relation was handled correctly by the direction rule but nothing pinned it. Separately one component test indexed update.mock.calls[0] without first asserting the call happened.
severity: minor
resolution: Added "owns a symmetric self-inverse relation rendered outgoing" to ownedRelations.test.ts, and an explicit toHaveBeenCalledTimes(1) before indexing so a stopped save fails with a clear assertion rather than an undefined index.
status: addressed
---

placeholder
