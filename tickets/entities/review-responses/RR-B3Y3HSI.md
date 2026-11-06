---
id: RR-B3Y3HSI
type: review-response
title: No integration test across room and autosave
finding: useBodyCollab tests fake both the room and autosave, and the autosave conflict tests stub ownsContent. None of the save-coordination defects can be seen through these fakes.
severity: minor
resolution: Added useBodyCollab.autosave.test.ts, which wires useBodyCollab to the real useAutoSave as EntityDetail does. It found and fixed a redundant PATCH after a 412 resend.
status: addressed
---
