---
id: RR-O2CS29
type: review-response
title: Stranded case does not exercise the stranded branch
finding: 'internal/realtime/busy_test.go:187-192 renames planning to thinking in the whole YAML. WorkingStates then lists only thinking, so ListWaiting never returns the stored planning channel, and busy.go:227-229 (`cur.stranded`) is never reached. The test pins the store filter, not the stranded guard. In fact no stored state that ListWaiting returns can be stranded, so the guard is dead defensive code. Fix: either drop the guard with a comment that the query only lists declared states, or keep it and pin it with a fake ListWaiting that returns a channel in an undeclared state. Rename the test case so it says what it covers.'
severity: minor
resolution: The busy query lists only declared working states, so the stranded guard was unreachable; removed it, renamed the case to say the stored state is no longer declared, and noted in godoc that the State lookup drops stranded rows anyway.
status: addressed
---
