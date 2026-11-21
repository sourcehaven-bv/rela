---
id: RR-IG9MQ20
type: review-response
title: 'Stored run titles are not re-checked after a visible: change'
finding: 'The visible: check of a title runs when flows.yaml loads; a run started before a later grant keeps a title that names the property.'
severity: significant
resolution: 'The default title no longer stores the subject''s title: the run stores the flow label, and the view adds the subject''s title as the reader reads it. A title: template is checked against visible: at load, and the docs say that its copy is kept like an output''s.'
status: addressed
---
