---
id: RR-2ZYS3G
type: review-response
title: Outgoing related() from a face-restricted candidate reads draft edges
finding: An outgoing first hop reads the candidate's default-state tail, which holds its default face's content edges. A reader granted only policy@published matched on draft-only edges and learned a far property value.
severity: significant
resolution: 'GateTraversal now takes the candidate type and refuses an outgoing first hop when the principal''s read of the candidate is face-restricted (ErrTraversalUnsupported). Test: TestGateTraversal_OutgoingFromFaceRestrictedCandidateIsRefused; appbuild test asserts the candidate type reaches the gate.'
status: addressed
---
