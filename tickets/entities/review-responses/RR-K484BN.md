---
id: RR-K484BN
type: review-response
title: normalizeJSONNumbers is copy-pasted into the test and can drift from pgstore
finding: 'roundtrip_test.go:128 reimplements normalizeJSONNumbers ''so the test does not depend on the build-tagged pgstore package''. Two problems: (1) it''s a fork that can silently diverge from pgstore/entity.go:538''s real logic — the test is only meaningful if it matches EXACTLY; (2) the real version recurses via normalizeJSONMap and the copy inlines the map loop. ''Close'' is how invariance bugs hide. A forked oracle is not an oracle. FIX: extract the normalization into a non-build-tagged shared helper (e.g. internal/storeutil) that both pgstore and the test import, so there is one source of truth.'
severity: significant
resolution: The forked normalizeJSONNumbers copy is GONE. canonical.normalize() now handles raw json.Number directly (folding it the same way pgstore does), so the cross-backend test feeds the pg arm RAW UseNumber output and lets canonical do the folding — no reimplementation of pgstore's logic in the test at all. There is no longer a forked oracle to drift.
status: addressed
---
