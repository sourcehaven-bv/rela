---
id: RR-AWSR7MH
type: review-response
title: Dark mode button text may not contrast with a custom accent
finding: The button text uses --card-bg on --accent-color (and --dark-card-bg on --dark-accent-color). An operator palette with a pale accent gives low contrast.
severity: minor
reason: Both colors come from the operator palette, the same pair the accent bar and links already rely on. The default palette passes; rela does not judge operator colors.
status: wont-fix
---
