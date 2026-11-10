---
id: RR-VWJACBI
type: review-response
title: Guard verdicts shown for an unreadable subject
finding: views attached branch Holds and Failed, evaluated on the raw subject, even when the caller could not read the subject.
severity: minor
resolution: views drops the branches when the subject read is nil.
status: addressed
---
