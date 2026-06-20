---
id: RR-URBR6S
type: review-response
title: Cross-backend test simulates JSON path but never feeds divergent inputs
finding: 'roundtrip_test.go is the ''linchpin guarantee'' but only feeds values that survive both paths: plain ints, 1.5, string lists, string-keyed nested maps. It contains NONE of: 2.0, a date, time.Time, map[any]any, a control char in a value, uint64, large/overflowing int, negative numbers, deeply nested, null inside a list. Every one of those is a real divergence (the criticals) the test would have caught if present. A test that only feeds values the author already handled isn''t proving invariance. ALSO the fs simulation decodeViaYAML (line 88) does yaml.Marshal then yaml.Unmarshal of an already-Go-typed map — NOT faithful: the real fs path parses a YAML STRING the user wrote. Marshaling float64(2) won''t reproduce the 2.0-literal-stays-float behavior. FIX: decode from raw YAML text literals; seed the test with C1-C5 cases as regression fixtures; make it property-based/fuzz (L3).'
severity: significant
resolution: 'Rewrote roundtrip_test.go: each case is now RAW frontmatter TEXT decoded via the real yaml.Unmarshal (faithful to fsstore.parseDocument), and the pg arm marshals to JSON + decodes with UseNumber (faithful to pgstore). Added every review-found divergence as a regression fixture: 2.0, dates, datetimes, non-string-keyed maps, control-char-in-value, large uint. Added FuzzCrossBackendDecode (966k execs clean) which itself FOUND a new divergence (leading-zeros->float precision) that''s now fixed and seeded. The simulation no longer marshals already-Go-typed values.'
status: addressed
---
