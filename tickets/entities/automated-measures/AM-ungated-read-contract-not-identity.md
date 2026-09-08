---
id: AM-ungated-read-contract-not-identity
type: automated-measure
title: Seam test asserts the ungated read contract, not store face identity
description: 'TestScriptReadSeam_PolicylessProjectStaysUnrestricted pins the policy-less script read path by asserting the reader is a *visibility.UnrestrictedReader AND that a read passes through to the store, instead of comparing face identity with app.store. Identity assertions break on any behaviour-preserving rename of the ungated wrapper (as #1208/TKT-1WV50C did), reddening CI on every open PR while the security contract is in fact intact. Asserting the observable contract keeps the fail-open regression covered without coupling to which object the wiring returns.'
kind: test
location: internal/dataentry/failclosed_test.go (TestScriptReadSeam_PolicylessProjectStaysUnrestricted)
status: active
---
