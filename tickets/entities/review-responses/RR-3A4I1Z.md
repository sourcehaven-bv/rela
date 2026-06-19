---
id: RR-3A4I1Z
type: review-response
title: 'Hash collision: separator bytes can appear in property values'
finding: 'canonical.go:43-52,82-85 assert separators 0x1c-0x1f ''cannot be produced by record content''. FALSE — reproduced. yaml.v3 round-trips a string containing \x1f losslessly, so a property value CAN contain these bytes. Two distinct entities then produce byte-identical canonical forms: props {a:''p'', b:''q''} vs a single property whose value embeds \x1e b \x1f s:q — both hash to the same SHA-256. A collision means If-Match silently accepts a wrong record / pull treats two records as identical = silent data divergence, the exact failure the package prevents. LLM-authored values emit control chars more than expected. TestHashEntity_NoFieldCollision (canonical_test.go:151) only tests a delimiter in the id field, never a delimiter inside a property value crossing a boundary. FIX: make encoding unambiguous regardless of content — length-prefix every field/value (TLV) or stream into hash.Hash with length prefixes (see L1). Delimiter-by-assumption is never safe for arbitrary input.'
severity: critical
resolution: 'Rewrote the hash to a length-prefixed streaming encoding (writer type in canonical.go): every variable-length field/value/key is written as an 8-byte big-endian length followed by its bytes, streamed into sha256.New(). Delimiter-by-assumption removed entirely. A value can no longer smuggle a separator to forge another structure. Regression test TestHashEntity_PropertyValueCollision asserts the exact collision pair from the finding (props {a:p,b:q} vs a:''p\x1eb\x1fs:q'') now hashes distinctly, plus a key/value boundary case. Also covered by FuzzCrossBackendDecode (966k execs clean).'
status: addressed
---
