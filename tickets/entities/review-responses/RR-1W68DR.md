---
id: RR-1W68DR
type: review-response
title: Lifecycle delete has no state CAS so a racing reopen bypasses from and retention
finding: B3 checks state (step 9) and retention (step 11), then calls Store.Delete, which takes no expected state or rev (sqlchannels Delete). A concurrent reopen or post lands in between and the delete still runs. Give Delete an expect State (or rev) like SetLifecycle and map a mismatch to 409 conflict.
severity: minor
resolution: Store.Delete takes the expected state and rev. A mismatch answers 409 conflict.
status: addressed
---
