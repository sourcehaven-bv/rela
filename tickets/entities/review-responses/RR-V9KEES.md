---
id: RR-V9KEES
type: review-response
title: acl.yaml loading paths untested (present-valid and malformed branches)
finding: All tests used a temp dir with no acl.yaml, so only the os.ErrNotExist fall-through was covered. The valid-policy path (policy flows into roles_matrix) and the malformed-yaml error branch — the ticket's own focus area — were untested.
severity: significant
resolution: Added TestBuild_ACLPresent_RolesMatrixRenders (writes a valid acl.yaml, asserts the role×verb matrix renders with both roles, not the no-policy note) and TestBuild_ACLMalformed_FailsLoud (writes broken YAML, asserts a wrapped fail-loud error mentioning acl.yaml).
status: addressed
---
