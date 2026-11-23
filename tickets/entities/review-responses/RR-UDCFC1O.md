---
id: RR-UDCFC1O
type: review-response
title: Untested world, decimal and agreement paths
finding: No test covered the world lookup miss, decimal values with mixed scales, agreement with the card endpoint, or the wiring of IdentityBinder and CardFigures over one svc.
severity: significant
resolution: Added TestCardFigure_UnknownDefaultWorld and TestCardFigure_DecimalValues. TestCardFigure_Displays uses the seed of TestDashboardCard_Displays and asserts the same values. The wiring was verified end to end on rela-server (implementation checklist evidence).
status: addressed
---
