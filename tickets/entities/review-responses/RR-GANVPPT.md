---
id: RR-GANVPPT
type: review-response
title: Declared mime refused types sniffing cannot fingerprint
finding: An exact match against the sniffed type refused correct declarations such as text/csv (sniffs text/plain) and Office documents (sniff application/zip).
severity: significant
resolution: 'sniffContradicts: exact match only for fingerprinted types (pdf, png, jpeg, ...); other types pass when the sniff is generic. TestSniffContradicts and a text/csv action test.'
status: addressed
---
