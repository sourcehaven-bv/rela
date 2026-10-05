---
id: RR-GUJG3Q
type: review-response
title: No test that a cached command render never reaches a principal denied that face
finding: '[security] GetCached is keyed on entry address + content hash only. Safe because the face gate runs first, but the order is not pinned for the cache.'
severity: minor
resolution: 'Subtest ''render cache does not serve a denied face'': bob renders policy_cmd over the draft, then alice''s request is the uniform 404 identical to a missing entity.'
status: addressed
---
