---
id: RR-8VKE
type: review-response
title: Test corpus + API doc gaps (hex roundtrip, over-arity table-arg, Record.Type pinning, fuzz seeds, Issue.Err set, concurrent depth)
finding: 'Bundle of small test/doc gaps: (a) TestCompile_NumberLexicalForms only checks compile, add eval roundtrip for 0xFF → 255; (b) no reject test for over-arity call with table-arg as extra (e.g. has_relation(''x'', {a=1}, {b=2})); (c) Record.Type()==RecordType{} (empty) is intentional but no test pins it — add a one-liner; (d) FuzzCompile seed corpus could include the hostile-input shapes from TestCompile_RecoversParserPanics; (e) Issue.Err is `error` but always *ParseError or *CompileError — document the closed set in lint.go doc comment; (f) TestProgram_Eval_Concurrent uses a trivial program (v < 100) — add a more complex shape (has_role + entity.status) to catch regressions where someone caches sig lookup on a callNode.'
severity: minor
resolution: 'Bundle addressed: (a) TestEval_NumberLexicalFormsRoundtrip verifies 0xFF→55, 1e10, 1.5e-3 evaluate correctly (not just compile); (b) testdata/reject/over_arity_table_arg.lua exercises has_relation(''x'', {a=1}, {b=2}); (c) TestRecord_TypeIsEmptyRecord pins that Record.Type() returns RecordType (without inspecting fields); (d) FuzzCompile seed corpus extended with the hostile shapes from TestCompile_RecoversParserPanics; (e) Issue doc comment now says ''Err is one of *ParseError or *CompileError; use errors.As to inspect''; (f) TestProgram_Eval_Concurrent_Complex added with a has_role + entity.status program.'
status: addressed
---
