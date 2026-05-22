---
id: RR-S84L
type: review-response
title: Risk R3 lacks a test — add TestCompile_RecoversParserPanics
finding: 'R3 says we''d wrap parse.Parse in recover() and convert to ParseError. The plan documents this as ''acceptable containment'' but the test plan never asserts the recover wrapper exists or works. Add TestCompile_RecoversParserPanics: constructs a Compile call whose error path is forced via a deliberately panicky reader (io.Reader returning a panic on Read). Even if no real Lua input crashes the parser today, the test pins the recover discipline so future code can''t accidentally remove it.'
severity: minor
resolution: TestCompile_RecoversParserPanics added in AC6 alongside FuzzCompile. Test forces a deliberately panicky io.Reader to exercise the recover() wrapper around parse.Parse; pins the discipline against accidental removal.
status: addressed
---
