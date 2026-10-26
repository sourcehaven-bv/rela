---
id: RR-VUWRN1
type: review-response
title: Claim writes the poster's identity as the current transport names it
finding: 'The plan says `{{message.author}}` "is what `QueryIdentity.ID()` compares" (lines 262-266). That holds within one transport only. `Author.User` is `p.User`, which the web edge resolves through `principal_property`. CLI and MCP use `ResolveDirect` and keep the raw identifier (`acl/identity.go:168-175`; `predicatefns/currentuser.go:80-88`). A person whose CLI or MCP identity is an email therefore claims a value their browser session does not match. Change: document this beside the claim example (identity configuration must agree across transports). Add a claim row over an MCP-stamped principal in `TestSession_HandoffAndClaim`.'
severity: minor
resolution: Claim identity documented (transport-resolved Author.User; config must agree across transports); AC 20 adds a claim over an MCP-stamped principal.
status: addressed
---
