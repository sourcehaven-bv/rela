---
id: RR-TRYPDZ
type: review-response
title: _remove does not trim addresses
finding: 'piles_handler.go: add trims whitespace before parsing, remove does not, so removing " TKT-1" is a silent no-op.'
severity: nit
resolution: _remove trims each address like add. TestPiles_RemoveTrimsAddresses.
status: addressed
---
