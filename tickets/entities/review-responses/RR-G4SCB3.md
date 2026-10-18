---
id: RR-G4SCB3
type: review-response
title: no_principal hint names $USER on remote MCP too
finding: UserText appends noPrincipalHint (usertext.go:21) for every transport. A remote MCP caller without identity is told to set $USER, which is wrong there. Let the transport add the hint, or pass the transport kind in.
severity: minor
resolution: The hint left denialText. It is now realtime.NoPrincipalHint plus realtime.IsNoPrincipal. The MCP channel handler adds it only when ChannelDeps.LocalIdentity is set, which only stdio sets. Tested by TestChannelFailed_NoPrincipalHint and a LocalIdentity check in the remote test.
status: addressed
---
