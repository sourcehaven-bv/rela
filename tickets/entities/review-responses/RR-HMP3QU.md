---
id: RR-HMP3QU
type: review-response
title: Preset null value is unspecified
finding: 'props: { decision: ~ } is not covered. A JSON null in a post is an absent property (internal/realtime/validate.go:50). Treat a null preset value the same way, so a required property then fails, or refuse it explicitly. Add the case to TestButtons_PropsProblems.'
severity: nit
resolution: A null preset value is an absent property, as a JSON null is in a post, so a required property then fails. TestButtons_Load and TestButtons_PropsProblems gain the case.
status: addressed
---
