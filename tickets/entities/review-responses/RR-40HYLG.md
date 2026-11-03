---
id: RR-40HYLG
type: review-response
title: List integer property validates but sums to 0
finding: 'checkIntegerProperty only checked the type. A list: true integer arrives as an array, numericValue returns undefined, so the total shows 0 and a currency field nothing.'
severity: significant
resolution: checkIntegerProperty rejects list properties; Go test cases for column_total and card currency.
status: addressed
---
