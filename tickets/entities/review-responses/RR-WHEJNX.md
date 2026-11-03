---
id: RR-WHEJNX
type: review-response
title: Command qualifier default not normalised
finding: 'matchesPage compared the qualifier to Name() so qualifier=default never matched a dashboard: true command although the dashboard API treats default as the default dashboard.'
severity: significant
resolution: commands.go maps the qualifier default to the empty qualifier; TestNamedDashboard command scope test asserts it.
status: addressed
---
