---
id: RR-L4TT3L
type: review-response
title: No unit tests for appSdk.ts or AppHostView.vue (the two most security-sensitive client files)
finding: 'relaBridge.test.ts is solid, but the in-iframe SDK source generation, the handshake message validation, and the host-side port wiring have ZERO unit coverage. Untested: that appSdkSource''s handshake listener rejects messages whose type !== handshakeType (appSdk.ts:68); that the ''port || !ev.ports[0]'' guard blocks a second port hijacking the channel (appSdk.ts:69); the injectAppSdk head-vs-no-head branching. The e2e covers the happy path but no adversarial case (spoofed handshake, second postMessage). FIX: add appSdk.test.ts (jsdom) asserting handshake validation + injection branches, and an AppHostView test or e2e for the spoofed-message case.'
severity: significant
resolution: 'Added frontend/src/bridge/appSdk.test.ts (7 tests): injectAppSdk placement (head/headless/comment-bypass) + appSdkSource handshake behavior (exposes one rela method per allow-listed method; ignores non-parent source; ignores wrong type; first-port-wins). 16 bridge tests total now pass. AppHostView''s bridge path is also exercised end-to-end by e2e/tests/apps.spec.ts.'
status: addressed
---
