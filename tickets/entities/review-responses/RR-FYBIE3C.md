---
id: RR-FYBIE3C
type: review-response
title: 'Design: gated read of ours breaks the merge'
finding: Reading ours through the gated reader makes redacted fields look cleared and hidden rows look deleted.
severity: significant
resolution: 'Plan: ours is a raw write-prep read; the manager still authorizes the write as system:peer:<name>.'
status: addressed
---
