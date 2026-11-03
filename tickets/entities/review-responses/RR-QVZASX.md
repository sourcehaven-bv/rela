---
id: RR-QVZASX
type: review-response
title: Faces and grant conditions untested on the backends
finding: No conformance case covers faced related types, related FaceIn, or Require/PickRequire with FaceIn or HasInbound (also raised by the security review for sqlite).
severity: significant
resolution: 'storetest faces_and_grant_conditions: world draft-first, related FaceIn, Require FaceIn, Require HasInbound, PickRequire FaceIn on a draft winner, PickRequire HasInbound. Passes on memstore, fsstore, sqlitestore and pgstore.'
status: addressed
---
