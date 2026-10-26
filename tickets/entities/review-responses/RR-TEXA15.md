---
id: RR-TEXA15
type: review-response
title: The plan names tests that do not exist, and the no_session check order is unstated
finding: '`TestRefusedKeys` and `TestParse_ValidTestdata` do not exist. The refused-key and valid-testdata checks live in `realtimecfg/load_test.go` (valid.yaml is read at line 44). Change: name the real tests or create them on purpose. Check `no_session` from config before the anchor read in `Service.Session`. That saves the read, and the answer does not depend on whether the anchor exists.'
severity: nit
resolution: Real test names used (TestParse_SessionInvalid, TestParse_Golden, openapi channels test); Service.Session answers no_session from config before the anchor read (AC 22).
status: addressed
---
