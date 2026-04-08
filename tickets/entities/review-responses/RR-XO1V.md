---
id: RR-XO1V
type: review-response
title: stringifyFilterQuery collision when values contain & or =
finding: 'The signature builder concatenates sorted key=value pairs with & and = as separators without escaping. Vue Router''s LocationQuery stores DECODED keys/values, so a value like ''foo=bar'' or ''a&b'' is plausible (text filter input). Two distinct queries can produce the same signature: {filter[a]: ''x'', filter[b]: ''y''} and {filter[a]: ''x&filter[b]=y''} both stringify to ''filter[a]=x&filter[b]=y''. If lastWrittenSig coincidentally equals an external nav''s signature, the watcher skips the read and the UI desyncs from the URL. Fix: use encodeURIComponent on key and value, or use JSON.stringify on a sorted entries array.'
severity: critical
resolution: 'filters.ts stringifyFilterQuery now serializes via JSON.stringify on a sorted entries array. Each value is preserved as-is (string, null, or string array) so values containing & or = can''t collide with key boundaries. Added 2 regression tests: (a) two-key query vs one-key-with-& collision, (b) value containing ''='' collision. All existing tests updated/passing.'
status: addressed
---
