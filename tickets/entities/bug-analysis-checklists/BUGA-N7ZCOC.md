---
id: BUGA-N7ZCOC
type: bug-analysis-checklist
title: 'Analysis: Entity-anchored documents skip the face gate: a face-restricted reader can render a hidden face'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally: TestFaceGrant_AnchoredDocumentIsFaceGated failed before the gate: `POL-1@draft` rendered for a `policy@published` reader and the fake engine recorded the call
- [x] Minimal reproduction steps documented: in the bug body
- [x] Environment/conditions noted: memstore via facedApp; any backend, since the gap is in the handler

## Root Cause

- [x] Immediate cause identified (why1): no faceReadable call in resolveAnchoredDocument
- [x] Contributing factors found (why2-3): route-by-route face gating
- [x] Systemic cause explored (why4-5): no registry of address-accepting routes

## Fix Planning

- [x] Fix approach determined: call faceReadable after GetEntityState and answer the uniform 404
- [x] Regression test planned: TestFaceGrant_AnchoredDocumentIsFaceGated, HTML and export routes, renderer must not run
- [x] Related areas checked for similar issues: entity export had the same gap and is fixed in BUG-PLZDPR on the same branch
