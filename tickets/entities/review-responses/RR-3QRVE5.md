---
id: RR-3QRVE5
type: review-response
title: Parity matrix never exercises an address the grammar refuses
finding: Every fixture address parses, so the parse-failure branch was not compared.
severity: minor
resolution: 'Added cases for three refused addresses: parseEntityRef rejects each, and the resolver answers a clean miss.'
status: addressed
---
