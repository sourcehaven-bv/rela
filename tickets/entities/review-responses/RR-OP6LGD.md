---
id: RR-OP6LGD
type: review-response
title: 'Design: content-scoped edges leak across faces in entity views'
finding: 'Probing the fixture showed GET /_views/policy/POL-1?world=editorial lists both CTL-1 (the draft''s implements edge) and CTL-3 (the published face''s) in the implements section, while the entry''s relations map says implements: [CTL-1]. CTL-1''s implemented-by section in the published world shows POL-1 although only POL-1@draft implements it. The plan had no answer for a live flow that exposes an untracked bug.'
severity: significant
resolution: 'Plan updated: the per-face relation spec is written as a live assertion of the documented behaviour and marked test.fixme naming the defect as untracked, with the probe output as evidence in the PR report. No product change in this unit.'
status: addressed
---

Probing the fixture showed GET /_views/policy/POL-1?world=editorial lists both
CTL-1 (the draft's implements edge) and CTL-3 (the published face's) in the
implements section, while the entry's relations map says implements: [CTL-1].
CTL-1's implemented-by section in the published world shows POL-1 although only
POL-1@draft implements it. The plan had no answer for a live flow that exposes
an untracked bug.
