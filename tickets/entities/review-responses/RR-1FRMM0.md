---
id: RR-1FRMM0
type: review-response
title: History snapshot read does not check the face
finding: serveHistoryVersion checked only snap.Type; a HistoryReader ignoring ref.Face would serve another face under this face grant.
severity: minor
resolution: 'serveHistoryVersion returns the uniform 404 when snap.Face differs from the subject face. Test: TestHistoryVersion_RefusesASnapshotOfAnotherFace.'
status: addressed
---
