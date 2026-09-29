---
id: RR-MQG3VT
type: review-response
title: Full MCP wiring and face-restricted case untested
finding: Tests injected a fake world source and a fake face-hiding reader; nothing proved the real gate and real gated reader.
severity: significant
resolution: Added TestRemoteMCPDeps_FaceRestrictedReaderGetsTheFaceTheyMayRead (real acl.yaml, policy@concept grant) and TestMCPReadWorld_ThroughTheRouter (real router, attachACLRequest, real Declarative).
status: addressed
---
