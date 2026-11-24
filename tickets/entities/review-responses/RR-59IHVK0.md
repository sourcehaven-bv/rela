---
id: RR-59IHVK0
type: review-response
title: Dutch ordinals 1ste and 2de fail
finding: The first matching suffix e was stripped so 1ste became 1st.
severity: significant
resolution: Suffixes are tried longest first and the remainder must be digits; tests for 1ste mei and 2de juni.
status: addressed
---
