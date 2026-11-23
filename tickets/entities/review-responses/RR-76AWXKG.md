---
id: RR-76AWXKG
type: review-response
title: Average rounded twice
finding: numericFold returns display text already rounded; rounding again to the field scale gives wrong digits.
severity: significant
resolution: CardFigure takes the exact big.Rat from a shared fold helper and rounds once to the field.
status: addressed
---
