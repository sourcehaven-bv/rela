---
id: RR-D4NH3GW
type: review-response
title: A save that succeeds does not release the hold
finding: unsavedProps/contentUnsaved were cleared only by edit or revert. When save A of a field failed while a newer save B was queued, A's catch held the field; B succeeded but the hold stayed, so the field ignored every later server value with no error shown.
severity: significant
resolution: A successful save releases the fields it wrote (moveBase) and the body (mergeServerResponse). The fireDue/fireContent catch and moveBase skip a key with a newer save queued. Unit test 'releases a field once a later save of it goes through'; the property now checks that every held field shows an error, which fails with the fix reverted.
status: addressed
---
