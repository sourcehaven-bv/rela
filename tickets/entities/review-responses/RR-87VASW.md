---
id: RR-87VASW
type: review-response
title: Data-entry validator traversal ignored the caller's gate
finding: '[security] The data-entry validator answered related() through a gate frozen at build time rather than the request principal''s gate, so a hidden target could satisfy a rule.'
severity: minor
resolution: App.scriptTraversalGate derives the gate from the Declarative ACL; subtest 'no read gate on ctx' pins it.
status: addressed
---
