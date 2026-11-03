---
id: RR-TLZNJN
type: review-response
title: setRow probes the faced row and falls back to a bare-id read on NotFound instead of asking the metamodel
finding: 'internal/appbuild/realtime_outputs.go:126-145: for a family-anchored type with a set.face, every anchor of an unfaced attach_to type costs a failed faced read plus a zero-face read, on both Check and Write. For a faced anchor whose configured face row is missing, the code still issues a zero-face read, which CLAUDE.md says a faced type does not have. The outcome is right (not_found), but the face is chosen by probing and not by declaration. Fix: in outputTarget or setRow, drop the face only when metamodel.FaceOrderOf(anchor type) is empty (the type is known from the key or anchor resolution), and never fall back for faced types.'
severity: minor
resolution: setRow decides faced vs unfaced from the metamodel, no bare-id fallback, type must match the mark's AnchorType; TestSetRow_FaceFromMetamodel.
status: addressed
---
