---
id: RR-UR2MQI4
type: review-response
title: min/max can disagree with the card on a non-numeric value
finding: The capture skipped values that do not parse; the card picks by list sort order, so a hand-edited n/a showed on the card but not in the stored figure.
severity: minor
resolution: foldFigure refuses a fold when a row holds a value that is not a number, for every fold. TestCardFigure_NonNumericValueRefusesFold.
status: addressed
---
