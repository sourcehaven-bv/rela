---
id: RR-P6CG9Q6
type: review-response
title: Errors from prepare or land escape loadView
finding: read catches only fetch errors; a throw in prepare or land becomes an unhandled rejection and no error is shown.
severity: minor
resolution: Errors thrown by prepare or land come back as failed. Unit test added.
status: addressed
---
