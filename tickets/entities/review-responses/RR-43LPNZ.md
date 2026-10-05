---
id: RR-43LPNZ
type: review-response
title: Duplicate and comments fixmes were weak
finding: The duplicate fixme passed on any URL change; the comments fixme did not check the comment stayed off the published face.
severity: minor
resolution: Duplicate reads the landed id and asserts a draft face exists and no published face; comments also asserts the published thread is empty.
status: addressed
---

The duplicate fixme passed on any URL change; the comments fixme did not check
the comment stayed off the published face.
