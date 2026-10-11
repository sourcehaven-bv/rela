---
id: RR-ZATEUB
type: review-response
title: Backlink sources must be loaded per face
finding: Reusing loadDefaultFaceHeaders would list a mention written on a hidden face under a readable one.
severity: minor
resolution: 'The index is keyed by state (id, face). The resolver lists an id only if the face the world serves for it is one of its mentioning faces. Tests: TestResolver_MentionedBy (hidden draft face) and TestResolver_MentionedByCursorNamesNoHiddenFace.'
status: addressed
---
