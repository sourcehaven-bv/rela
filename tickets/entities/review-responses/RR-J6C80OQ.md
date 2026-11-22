---
id: RR-J6C80OQ
type: review-response
title: 'Duplicated whole-number decoder'
finding: 'ActionOrder.UnmarshalYAML copied Span.UnmarshalYAML, and neither error named a line.'
severity: minor
resolution: 'Both use a shared decodeWholeNumber helper whose error includes the YAML line.'
status: addressed
---
