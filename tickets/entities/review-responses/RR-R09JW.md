---
id: RR-R09JW
type: review-response
title: Encoded slashes in matched path segments undocumented
finding: 'internal/frontendroutes/routes.go:81-88 Match compares segments byte-for-byte: /form/edit_ticket/TKT%2F001 matches /form/:id/:entityId and captures entityId="TKT%2F001". No decode happens. Not wrong — no caller decodes — but worth a one-line godoc note so future consumers aren''t surprised.'
severity: nit
resolution: 'Match godoc now states: ''Path segments are compared byte-for-byte; percent-encoded characters are not decoded. /form/edit_ticket/TKT%2F001 matches /form/:id/:entityId and the captured entityId value would be the literal TKT%2F001.'' Added a subtest exercising the encoded-slash case.'
status: addressed
---
