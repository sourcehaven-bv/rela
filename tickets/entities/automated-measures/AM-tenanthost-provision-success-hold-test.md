---
id: AM-tenanthost-provision-success-hold-test
type: automated-measure
title: 'Test: a request racing the end of a provisioning run starts no second run'
description: TestProvision_NoRerunAfterSuccess holds a lazy request and a webhook between Acquire and start while the run succeeds, and asserts one Provision call. TestProvision_SuccessHoldEnds pins that the hold expires.
kind: test
location: internal/tenanthost/provision_test.go
status: active
---
