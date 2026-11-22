---
id: RR-KMCEDAJ
type: review-response
title: 'style and order without available_on are ignored'
finding: 'style and order on an action without available_on loaded without error and did nothing, unlike when.'
severity: significant
resolution: 'validateActionScope now refuses style or order without available_on; tests and guide updated.'
status: addressed
---
