---
id: RR-ELBHV2
type: review-response
title: Test gaps in faced-grant and identity checks
finding: 'Missing cases: *@draft beside a bare grant, faces: {} through the real adapter, a role relation that is also the membership relation, an undeclared inherit relation, joined identity and grant errors.'
severity: minor
resolution: Added each case to internal/acl/facedgrants_test.go and internal/appbuild/facedgrants_test.go.
status: addressed
---
