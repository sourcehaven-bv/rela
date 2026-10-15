---
id: RR-TTC4RD
type: review-response
title: Configure Dashboard silently edits default for unknown name
finding: With an unknown ?name= the Configure Dashboard screen edited the default dashboard without saying so.
severity: significant
resolution: The screen now shows a note (data-testid config-dashboard-unknown) naming the unknown dashboard and saying the default cards are shown.
status: addressed
---
