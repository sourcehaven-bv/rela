---
id: RR-MCWQ6S
type: review-response
title: Artifact glob uploads committed seeds alongside new crashers
finding: internal/**/testdata/fuzz/** doesn't distinguish the 12 committed regression seeds from new crashing inputs.
severity: nit
reason: 12 files of noise at current scale; the issue body's kind tags identify which targets actually crashed. Revisit if the corpus grows large.
status: wont-fix
---
