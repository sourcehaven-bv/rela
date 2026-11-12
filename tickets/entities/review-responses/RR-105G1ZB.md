---
id: RR-105G1ZB
type: review-response
title: Row-level resolve could settle entries on hidden fields
finding: remotesync.Resolve with an empty field resolves every entry of the row. A web caller who may not see one of the fields would settle its entry too (a write past the field gate), and the gate checked a different set than the write acts on.
severity: significant
resolution: The web routes name one entry exactly (local, field; empty field means the row-level entry only). remotesync gets an exact-entry mode; the handler gates that entry and the write acts on that entry alone.
status: addressed
---
