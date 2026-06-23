---
id: RR-L29M1S
type: review-response
title: In-iframe SDK accepts the port handshake from any window sender (no ev.source check)
finding: 'appSdk.ts:67-77 checks ev.data.type and ev.ports[0] but never ev.source === window.parent. A nested frame the app creates (allowed under allow-scripts) could race the parent''s handshake and, because ''if (port) return'' is first-port-wins (appSdk.ts:69), capture the SDK with a port it controls — MITMing every rela.* call. Low exploitability (timing race + the app is already trusted-ish), but this is the trust-boundary code. FIX: validate ev.source === window.parent in the handshake listener.'
severity: significant
resolution: appSdk.ts handshake listener now rejects any message whose ev.source !== window.parent before accepting the port, so a nested frame the app creates cannot race the host and MITM the bridge. Covered by an appSdk.test.ts case ('ignores a handshake from a source that is not window.parent'). e2e confirms the legitimate parent handshake still works.
status: addressed
---
