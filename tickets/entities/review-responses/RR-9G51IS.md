---
id: RR-9G51IS
type: review-response
title: Section-inheritance and effort rationale need strengthening
finding: 'Two weaker points in PLAN-DMQFRJ. (1) The plan justified ''no section-level widget inheritance'' as ''meaningless across mixed types''. That argument doesn''t survive the obvious counter-example -- a section of five string properties that all want textarea -- and render: inherits despite being arguably per-field too. (2) Effort estimated ''s'' on the grounds of ''four backend files, three frontend, following a thread merged two commits ago'', which omits the drift-guard tests, the attachments decision, the StatusControl rework, the hint-arm test, and correcting the ticket itself. TKT-HOIX1 was scoped as a single-axis change and produced 60 files and 14 review responses.'
severity: minor
resolution: '(1) Replaced the rationale with two stronger arguments: render: is a binary whose both values are valid for every field, so it can safely inherit, whereas a section-level widget: is a config-load ERROR on every field of a non-matching type -- inheritance that must be undone field-by-field is anti-inheritance; and validating a section-level widget requires per-field property types, i.e. exactly the metamodel-dependent context RR-4ICH8M moved the field-level check out of. Both go in docs/data-entry.md where the field-level-only rule is documented. (2) Re-estimated effort from s to m.'
status: addressed
---
