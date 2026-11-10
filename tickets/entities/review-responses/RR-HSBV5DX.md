---
id: RR-HSBV5DX
type: review-response
title: Registry does not know it is closed
finding: An Acquire during or after Close could insert a resident nothing closes.
severity: minor
resolution: 'Close sets a closed flag; Acquire returns ErrClosed and an open finishing after Close closes its value. Close waits for opens in progress. Test: TestRegistry_ClosedRefusesAcquire.'
status: addressed
---
