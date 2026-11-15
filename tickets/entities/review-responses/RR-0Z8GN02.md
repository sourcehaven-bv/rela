---
id: RR-0Z8GN02
type: review-response
title: NAT64 local-use and IPv4-compatible addresses not refused
finding: The always-on Lua HTTP destination check did not read the IPv4 address embedded in local-use NAT64 (64:ff9b:1::/48) and IPv4-compatible (::/96) addresses, so a refused IPv4 target was reachable through those forms.
severity: minor
resolution: 'Fixed in 6ed495060: the address check extracts the embedded IPv4 address from both forms and applies the IPv4 rules. Pinned by httppolicy_test.go.'
status: addressed
---
