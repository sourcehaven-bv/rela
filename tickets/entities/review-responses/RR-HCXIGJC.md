---
id: RR-HCXIGJC
type: review-response
title: No EntityDetail tests for body/title across a switch; real-time wait in toggle test
finding: The new EntityDetail test covered only checkbox toggles, which the old pin already handled, and waited with a real setTimeout(200).
severity: significant
resolution: Added body and title switch tests (the body test fails with the key and the pin both removed) and dropped the unneeded wait from the toggle test. A face-changing world switch is covered by the per-address instance and the faced assertTarget test.
status: addressed
---
