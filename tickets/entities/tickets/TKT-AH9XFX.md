---
id: TKT-AH9XFX
type: ticket
title: Reject cleartext http:// for plantuml_server_url except loopback
kind: enhancement
priority: low
status: review
---

## Problem

`app.plantuml_server_url` accepts `http://` for any host. The SPA
deflate+base64-encodes diagram source and points an `<img>` at
`<server>/svg/<encoded>`, so with an `http://` URL to an external host the
diagram source — potentially confidential business information — crosses the
network in cleartext.

Reported as [#1040](https://github.com/sourcehaven-bv/rela/issues/1040) from the
IB review of #1034. Severity: Low. Grounds: CONTROL-8-21 (missing TLS as a
network-service risk factor) and CONTROL-5-14 (information transfer to other
parties).

## Why not simply require https

A self-hosted PlantUML server on `localhost` or a container host over plain
`http` is a completely legitimate deployment, and a common one — PlantUML is
usually run as a local sidecar precisely so diagram source never leaves the
network. A blanket https-only rule would break those setups for no
confidentiality gain: on loopback there is no network segment to eavesdrop.

The risk is cleartext to an **external** party. So the rule should track that
distinction rather than the scheme alone.

## Approach

Require `https` in general; permit `http` only for loopback hosts (`localhost`,
`127.0.0.0/8`, `::1`).

Deliberately NOT extending the exemption to RFC1918 private ranges. A private
range is not the same trust boundary as loopback: traffic to `10.x` or
`192.168.x` traverses a real network segment with real other hosts on it, and
"private" says nothing about who else can see the wire — that is exactly the
LAN-eavesdropping case CONTROL-8-21 covers. An operator with a private-network
PlantUML server can still use TLS. Loopback is the only case where the cleartext
argument genuinely does not apply, because there is no segment.

Host parsing uses `url.Parse` + `Hostname()`, which strips ports, userinfo and
IPv6 brackets. A naive prefix/suffix check would accept
`http://localhost.evil.com`, `http://127.0.0.1.evil.com` and
`http://localhost@evil.com` — all three are covered by tests.

## Acceptance criteria

- AC1 `https://` to any host remains valid.
- AC2 `http://` to a loopback host (`localhost`, `127.0.0.1`, any `127.x`, `[::1]`,
with or without port) remains valid.
- AC3 `http://` to a non-loopback host is rejected with a message naming the reason.
- AC4 Hosts that merely *look* loopback (`localhost.evil.com`, `127.0.0.1.evil.com`,
`localhost@evil.com`) are rejected.
- AC5 Non-http(s) schemes and missing hosts keep their existing errors.
- AC6 The rule is documented where operators configure the knob.
