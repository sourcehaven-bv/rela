---
id: RR-AQEWVT
type: review-response
title: Manual-ID target type loads but fails with 500
finding: 'The validator accepts to: types with id_type manual; Convert passes no id, generateID fails with a plain error, ClassifyCreateError gives Unknown, 500. Preview and Offers say allowed.'
severity: significant
resolution: 'The metamodel validator refuses a to: type with id_type manual. Pinned by a TestTaskItemConversionValidation case.'
status: addressed
---
