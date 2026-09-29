---
id: AM-document-face-gate
type: automated-measure
title: Anchored documents apply the face gate and render the addressed face
description: On the faced policy fixture a policy@published reader gets the uniform 404 for a document over POL-1@draft (identical to a missing entity); granted readers get 200 with the addressed face rendered through the real Lua reader and the command renderer; an ID@face or bare faced id never answers 500.
kind: test
location: internal/dataentry/document_face_test.go (TestAnchoredDocument_FaceGate)
status: active
---
