---
id: MEAS-CLIENT-CEILING-ONLY-POLICY
type: automated-measure
title: A client ceiling applies when no role declares field grants
description: A policy whose only field restriction is a client_baselines ceiling still hides and redacts those fields; a test pins this.
kind: test
location: internal/appbuild/ceilingonly_test.go, internal/dataentry/ceiling_e2e_test.go, internal/acl/policy_test.go
status: active
---
