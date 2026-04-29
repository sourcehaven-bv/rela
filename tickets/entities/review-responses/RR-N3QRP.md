---
id: RR-N3QRP
type: review-response
title: AC2 source-slice content assertion is too weak
finding: 'Asserts the highlighted line contains ''x.field''. No assertion that surrounding context lines (3 above, 3 below) match the original file content. A regression where readSourceSlice opens the wrong file but happens to find a line with ''x.field'' would pass. Location: internal/validation/lua_scripterror_test.go:124-135.'
severity: nit
resolution: Source-slice assertion in TestLuaValidation_FileRuntimeErrorIncludesSourceSlice now requires len(Source)==7 (3 above + failing + 3 below) and confirms at least one HEADER-* and one FOOTER-* marker from the test fixture appears in the slice. A regression that opened the wrong file but happened to find an x.field line would now fail. Commit 7221fa2.
status: addressed
---
