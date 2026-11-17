---
id: RR-VR56SES
type: review-response
title: Offsets were not bounded like date_add
finding: $today+Nd accepted offsets larger than date_add's cap.
severity: minor
resolution: DateVariable rejects offsets above 1,000,000 days.
status: addressed
---
