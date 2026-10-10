---
id: RR-ZDMXNI
type: review-response
title: Backend override lost on SharedBase.Assemble and reassembly
finding: SharedBase.Assemble passes empty backendOverrides, so multi-tenant and reload assemblies on postgres would silently get kvpiles over the pg StateKV.
severity: significant
resolution: 'Plan: the backend is derived from the store by a build-tagged storePilesFor(st) helper (like storeUserStateFor), so every assembly path agrees. sharedbase_test asserts pg-assembled Services get pgpiles.'
status: addressed
---
