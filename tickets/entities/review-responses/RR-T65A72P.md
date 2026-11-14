---
id: RR-T65A72P
type: review-response
title: assertBudget skips padded calls and drops detail
finding: assertBudget compared only padded rows and measureBudget discarded the padded detail strings.
severity: minor
resolution: budgetCost keeps padDetail; assertBudget calls storetest.AssertSameCost on the padded costs.
status: addressed
---
