---
id: RR-VFH4YX
type: review-response
title: Peer batch ran before structural checks
finding: A gate fault turned a 400 or 422 request into a 500, and a rejected request paid the read.
severity: minor
resolution: validateRelationsModern collects edges during the structural pass and gates them in one batch afterwards.
status: addressed
---
