---
id: RR-C8ZDSF
type: review-response
title: Owner title must be the redacted title
finding: 'The owner and parent refs carry a title. If the title property is under a field-level visible: rule, the raw store header would leak it. The title must come from the visibility reader, with a test.'
severity: minor
resolution: 'Plan: parent title comes from the visibility reader''s redacted header; handler test with a visible:-restricted title.'
status: addressed
---
