---
id: RR-Z58MQO
type: review-response
title: 'count: rules counted derived pseudo-fields'
finding: A count rule counted derived pseudo-fields beside their sources, so one real field could satisfy a count twice.
severity: significant
resolution: count counts distinct real source refs; derived pseudo-fields carry Sources. Pinned by TestDerive_CountIgnoresDerivedFields.
status: addressed
---
