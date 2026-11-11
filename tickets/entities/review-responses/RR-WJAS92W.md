---
id: RR-WJAS92W
type: review-response
title: A written step is reported as a failure
finding: complete wrote the subject and then called Change. When Change then failed (guard error, a timer moved the run, or settle failed after the move) the caller got 409 or 500 although the values were saved.
severity: significant
resolution: Once the write has landed, Complete answers 200 and sets Completed.problem to guard, moved or settling, and logs the error. Errors before the write still refuse the step. The SPA shows a matching notice.
status: addressed
---
