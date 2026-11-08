---
id: RR-3UJVP3H
type: review-response
title: 'Design: open streams outlive key revocation'
finding: Removing a digest stops new requests only; an _events stream or live view opened with a peer key would keep running.
severity: significant
resolution: Streams are outside the peer route list (_events, _views, _collab), so every peer request is a bounded request and revocation holds within the reload interval. TKT-I8L1EQU notes that an _events trigger needs credential expiry first.
status: addressed
---
