---
id: RR-QLEF38
type: review-response
title: rate can raise the burst while lowering the average
finding: AtMost compares only per-second; 1000/h passes against 30/m but a bucket sized by count allows 1000 at once
severity: significant
resolution: AtMost also requires Count <= limit.Count; 1000/h is refused under 30/m.
status: addressed
---
