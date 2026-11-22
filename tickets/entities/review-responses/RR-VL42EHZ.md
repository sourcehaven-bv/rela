---
id: RR-VL42EHZ
type: review-response
title: "aboutWire accepts a label of spaces"
finding: "Load-time validation trims the link label, the wire projection did not, so a live reload could serve a link with a blank label."
severity: minor
resolution: "aboutWire trims the label like validateAbout; a test case pins it."
status: addressed
---
