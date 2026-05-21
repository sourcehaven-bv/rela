---
id: RR-POA2
type: review-response
title: Lua string semantics and nil-vs-false equality rules unspecified
finding: 'Plan does not address: (a) Lua strings are byte-strings; ''a\0b'' is a 3-byte string compared byte-equal. If Bindings.Vars holds Go string, this is fine — but say it. (b) Lua''s `nil == false` is `false` (distinct values). Plan''s Value type has both nil and bool but doesn''t spell out equality rules per type pair. Add a table in doc.go listing the eq/neq semantics for every (typeA, typeB) pair, matching Lua semantics for primitive types. Cover the surprising cases: nil==nil is true, nil==false is false, 1==1.0 (depends on RR-VI93''s resolution), '''' (empty string) == nil is false.'
severity: significant
resolution: 'Equality matrix table added to plan (''Equality semantics'' section): nil==nil true, nil vs anything else false, bool==bool Go ==, number==number float64 ==, string==string byte-equal including null bytes, record/list disallowed at compile time. AC10 + TestEval_EqualitySemantics pin this. <, <=, >, >= allowed only on two numbers or two strings; mixed pairs are compile-time type errors.'
status: addressed
---
