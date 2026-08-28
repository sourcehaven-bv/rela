---
id: RR-OYZR4P
type: review-response
title: isUniqueViolation string-matched the driver's error message
finding: Conflict detection matched on err.Error() text. Its own doc comment admitted this was a spike artifact and that 'a real backend would type-assert *sqlite.Error and check the extended result code' — but this is the shipping backend. A reworded message upstream would degrade ErrConflict into a generic error, and the stress test explicitly tolerates ErrConflict and nothing else from a racing create.
severity: minor
resolution: Now type-asserts *sqlite.Error and checks the extended result codes (SQLITE_CONSTRAINT_PRIMARYKEY 1555, SQLITE_CONSTRAINT_UNIQUE 2067), declared as named constants since the driver does not export them. The string check is kept as a fallback for any path that surfaces an error the assertion cannot see.
status: addressed
---
