---
id: RR-I1CZ
type: review-response
title: Origin allowlist must match scheme+host+port exactly, plan glosses over `null` and case
finding: 'Plan says ''parse and check scheme+host+port'' but doesn''t address: (1) `Origin: null` is a valid header value sent by sandboxed iframes / data URIs / file:// — must be rejected explicitly, not allowed through some default; (2) Origin matching is case-sensitive for scheme+host per RFC 6454, but uppercase IPv6 vs lowercase needs normalisation; (3) trailing slash on Origin value is non-standard but some clients send it; (4) port `:80` for http and `:443` for https are implicit and may be omitted by the browser.'
severity: minor
resolution: 'Plan updated: Origin matching uses url.Parse + explicit field comparison (scheme, hostname, port) with default-port normalisation (`:80` for http, `:443` for https). `Origin: null` is rejected explicitly. Case normalisation: lowercase scheme and hostname. Trailing slash is rejected as malformed (RFC 6454: Origin has no path).'
status: addressed
---
