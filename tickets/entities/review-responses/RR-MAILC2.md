---
id: RR-MAILC2
type: review-response
title: A template task passed validation when mail-templates.yaml was absent
severity: significant
status: resolved
resolution: Cross-config validation now treats an absent template declaration file as an empty registry whenever schedules reference templates, producing a named unknown-template error. It also rejects list-valued recipient address properties.
---

Syntactic schedule validation knew only that `template:` was present. The first
cross-config implementation skipped reference validation when the declaration
file was absent, postponing an author-time error until a child job executed.
