---
id: RR-C7R21B7
type: review-response
title: '[security] Peer awareness is unvalidated'
finding: '[security] A peer''s user.color goes into a CSS custom property used as background, so url(...) makes every co-editor fetch an attacker URL. user.name is client-asserted, so a writer can label its cursor as another user.'
severity: minor
resolution: 'peerCursor accepts a colour only when it is #rrggbb and cuts the name to 64 characters, non-string fields are ignored. Test: editorCollab.test.ts.'
status: addressed
---
