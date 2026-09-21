---
id: RR-7SJ7KN
type: review-response
title: The Lua negative tests' no-write assertion could not fail
finding: face_write_test.go asserted `mgr.lastCreateFace != "" || mgr.lastRelationFace != ""` to prove a refused call never reached the manager. Both fields are the zero value until something sets them, and a successful zero-face write also leaves them empty — so the assertion detected only 'a write happened AND carried a non-empty face'. A binding that parsed the options, called CreateEntity with a dropped (zero) face, and only then raised would pass cleanly. That dropped-face case is precisely the bug the ticket exists to prevent, so the assertion could not fail on the defect it was written to catch. The entitymanager-side equivalent was correct (it scans the store); only the Lua side was weak.
severity: significant
resolution: Added createCalls/relationCalls counters to mockManager and switched the assertion to require both are zero. The field comment now records why a face-valued assertion is insufficient, so the next person does not revert it. Verified the strengthened test still passes and the counters increment on real writes.
status: addressed
---
