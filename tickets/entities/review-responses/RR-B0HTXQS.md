---
id: RR-B0HTXQS
type: review-response
title: Compact loading and stranded show no summary at wide widths
finding: The summary is display:none outside the container query and the strip needs a current state, so loading and stranded compact headers at 32rem and wider would show only the label and the toggle.
severity: significant
resolution: A --no-context modifier, bound to !showContext, shows the summary in compact. AC 6 and tests cover both statuses.
status: addressed
---
