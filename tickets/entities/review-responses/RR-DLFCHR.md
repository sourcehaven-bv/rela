---
id: RR-DLFCHR
type: review-response
title: errACLListQuery wrapping / writeGateError mapping preservation for _position not pinned by AC8
finding: readableSubset wraps gate errors in errACLListQuery so handleV1EntityPosition's writeGateError mapping (canceled→silent, deadline→504) fires. AC8 only pins filtering behavior via TestACLPosition_SearchScopeGated, not the error mapping; the new gated executeQuery must preserve the exact wrapping or a cancellation on /_position?scope=search starts emitting a 500 body where it used to emit nothing.
severity: significant
resolution: 'Plan rev 2 AC7b: executeQuery preserves errACLListQuery/errListLoad wrapping (unit-assert errors.Is); canceled-context on /_position?scope=search stays silent, deadline → 504. writeGateError mapping fires identically for both consumers.'
status: addressed
---
