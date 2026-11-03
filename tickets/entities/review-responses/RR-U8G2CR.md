---
id: RR-U8G2CR
type: review-response
title: Read-only clients never see waiting entries
finding: 'Waiting roles are held for CapabilityUpdate, so the client ceiling applies. An agent on a read-only token always gets an empty inbox, as read_session answers today. Fix: add one line to the identity table in docs/realtime.md so agent operators do not debug an empty inbox.'
severity: nit
resolution: 'Identity table in docs/realtime.md gains a read-only token row: waiting roles need update, so the client ceiling always yields an empty inbox, as waiting_on_me.'
status: addressed
---
