---
id: RR-RV39AZ
type: review-response
title: pick_one options read a second metamodel snapshot
severity: significant
status: addressed
finding: 'nextActionOptions called a.Meta() inside a loop -- a fresh atomic.Pointer load per call, and a different snapshot from the one executeQuery used to resolve the ACL scope. A config reload landing mid-resolve would label options against a metamodel that did not match the one that gated them. Violates the capture-state-once rule in the root CLAUDE.md.'
resolution: 'One a.State() snapshot is taken above the loop and s.Meta used throughout.'
---
