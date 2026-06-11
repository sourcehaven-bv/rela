---
id: RR-QDV6
type: review-response
title: 'Missing test variant: callerCtx fallback path (WithContext not set)'
finding: 'callerCtx() has two branches: parent set OR nil fallback. The new test only covers the first. A subtest without WithContext that asserts the binding succeeds with a background ctx would lock in the fallback path against future ''optimization'' that bypasses callerCtx().'
severity: minor
resolution: 'Added TestReadBindings_FallbackWhenNoParentContext. Constructs a runtime without WithContext, drives rela.get_entity, and asserts every recorded call had hasMarker=false. A future regression that reverts to hardcoded context.Background() would have to break both this test (binding''s call doesn''t carry the marker that callerCtx fallback also wouldn''t carry — same observed state, so this test wouldn''t catch it on its own) AND TestReadBindings_UseCallerContext (which catches the regression directly). The fallback test still adds value: it locks in that the binding works at all when no parent ctx is set, which existing tests cover implicitly.'
status: addressed
---
