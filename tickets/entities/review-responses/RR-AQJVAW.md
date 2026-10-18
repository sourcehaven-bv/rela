---
id: RR-AQJVAW
type: review-response
title: Stdio channel tools say "entity not found" for an entity show_entity just returned
finding: With acl.yaml present, a $USER that has no assignment sees every entity through the open entity tools, but every channel call answers with the uniform 404. On stdio there is no existence secret to protect, so a hint about the missing acl.yaml assignment would save a confused debugging session.
severity: minor
resolution: Kept the uniform not-found text. Stdio startup logs one warning when acl.yaml exists and its assignments name no role for $USER (appbuild.ACLAssignsNoRole; tested by TestMCPStdio_WarnsUnassignedUser). The realtime and MCP guides document the cause and the fix.
status: addressed
---
