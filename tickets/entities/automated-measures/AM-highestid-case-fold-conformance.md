---
id: AM-highestid-case-fold-conformance
type: automated-measure
title: 'Conformance test: HighestID folds prefix case and parses digits only'
description: Every store backend must count a mixed-case id such as Req-9 for the prefix REQ and read the suffix as digits only. The entitymanager test creates after a mixed-case id and expects the next number. Catches a backend whose HighestID compares the prefix bytewise or parses a leading digit run (BUG-RLBXTG5).
kind: test
location: internal/store/storetest/query.go (HighestIDFoldsPrefixCase, HighestIDCountsDigitsOnly) and internal/entitymanager/idmint_test.go (TestCreate_MintsPastMixedCaseIDs)
status: active
---
