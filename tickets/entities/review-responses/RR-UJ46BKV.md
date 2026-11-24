---
id: RR-UJ46BKV
type: review-response
title: Client and server disagree on unset state
finding: SPA read unset as off while the server used the machine entry value.
severity: significant
resolution: _toggles now carries checked and to from the server; the SPA renders only from it.
status: addressed
---
