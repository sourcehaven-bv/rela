---
id: BUG-T6HNKS
type: bug
title: Conflict routes are not ACL-gated (content disclosure)
description: Conflict routes skip the read gate and show the diff of a hidden entity, including hidden field values.
priority: high
status: backlog
---

## Description

The data-entry conflict routes skip the read gate. In the TKT-0M4H0S golden
corpus, the viewer gets the diff of hidden TKT-4, including the hidden `secret`
value and its content (`conflict-hidden.viewer`). `GET /_conflicts` also lists
the path of TKT-4.

## Fix

Row-gate the conflict list and detail through the visibility reader (a hidden
entity is a 404), and redact fields in the diff. Gate resolve on write
authorization. Pin with corpus cases.
