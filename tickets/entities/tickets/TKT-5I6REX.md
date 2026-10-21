---
id: TKT-5I6REX
type: ticket
title: End SSE streams when the verified token expires
kind: enhancement
priority: medium
effort: s
started: "2026-10-20"
completed: "2026-10-20"
status: done
---

## Description

An SSE stream outlives the JWT that opened it. With live views (TKT-HJ5Z9Y) a
stream keeps receiving `live:changed` frames computed for the connecting
principal, even after its IdP claims are revoked. Carry the assertion expiry on
the verified principal (or the stream), and end the stream at expiry so the
client reconnects with a fresh token. Raised in the TKT-HJ5Z9Y design review.
