---
id: RR-KX7AHW5
type: review-response
title: Backreference replacement depends on standard_conforming_strings
finding: '''\\1\\2'' becomes chr(1) when standard_conforming_strings is off, and the cast then fails.'
severity: minor
resolution: Written as an E'' string, which reads the same under either setting.
status: addressed
---
