---
id: RR-WWZKJK
type: review-response
title: Test gaps on the MCP path
finding: No MCP tests for an unreadable named face, face_required on update and accept, the real SDK transport, gate failures, the reload guard, or provisioning
severity: significant
resolution: Added TestMCPComments_UnreadableFaceIsNotFound, face_required for update and accept in TestMCPComments_Faces, TestCommentTools_RunAsTheHTTPCaller (real tools/call through HTTPHandler; the write runs as the stamped caller), TestCommentFailure_OpError, TestCommentTools_AfterAReloadDropsComments, and the provisioning subtest.
status: addressed
---
