---
id: RR-0LDG0R
type: review-response
title: Guard failures give counts without line numbers
finding: A file over its count reported only the numbers, so the developer had to find the new read by hand.
severity: significant
resolution: zeroFaceReads returns token.Position values; growth messages list the line of every read in the file.
status: addressed
---
