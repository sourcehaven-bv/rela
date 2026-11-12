---
id: RR-JH725LL
type: review-response
title: Removed reply time survives in last_reply_at
finding: A removed newest reply's time stays in the parent's last_reply_at.
severity: nit
reason: 'By design (acceptance criterion): a removal changes neither count nor time; the seq and neighbours already bound that time.'
status: wont-fix
---
