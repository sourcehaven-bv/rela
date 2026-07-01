---
id: RR-O50E4R
type: review-response
title: B5 reports 'not an enum' for a typo'd (absent) options field — misleading diagnosis
finding: 'B4 only inspects Fields/Visible, not Options. So an options: grant naming a field that doesn''t exist on the type falls into B5''s !isEnum branch and reports ''names field X which is not an enum'' — when the field isn''t there at all. Wrong diagnosis. Fix: B5 should call m.HasField first and report an undeclared-field finding for an absent field, reserving ''not an enum'' for a real-but-non-enum field.'
severity: minor
resolution: 'B5 now calls m.HasField first: an absent options field reports B4-undeclared-field (typo), reserving B5-options-non-enum for a real-but-non-enum field. Added TestAudit_B5_AbsentField_ReportsUndeclaredNotNonEnum.'
status: addressed
---
