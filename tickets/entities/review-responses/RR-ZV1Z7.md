---
id: RR-ZV1Z7
type: review-response
title: TestLuaValidation_ContractErrorMissingMessageField is two tests in one
finding: 'First half tests return { severity = ''error'' } and asserts 0 ScriptErrors via t.Logf, ''documenting'' the pre-existing bug the plan listed as out-of-scope. Second half tests array-element missing message. Test name promises AC7 case 2; what it actually tests is ''the pre-existing bug still exists, and also array elements work.'' Location: internal/validation/lua_scripterror_test.go:271-328.'
severity: minor
resolution: Renamed TestLuaValidation_ContractErrorMissingMessageField to TestLuaValidation_ContractErrorArrayElementMissingMessage and removed the first half (which used t.Logf to document a pre-existing bug already noted as out-of-scope in PLAN-KAK2R). The test now focuses solely on the array-element missing-message contract error. Commit 72347d2.
status: addressed
---
