---
id: RR-7RN4ZTS
type: review-response
title: Create judged before automation and update after
finding: createCore applies strict before on-create automation; updateCore applies it after.
severity: significant
reason: The create is stored before automation runs (BUG-ZWTDH9 CAS design), so judging after automation would need a second write path. Documented in the CLI reference with the template-default workaround.
status: wont-fix
---
