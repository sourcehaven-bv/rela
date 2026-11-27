---
id: RR-IQ3VARD
type: review-response
title: OnChange send can block Start if a test stops reading
finding: eventsChan holds 10 batches; if a test stops reading while batches keep coming, Start blocks in OnChange and Stop cannot end it. Predates the diff.
severity: nit
reason: Pre-existing; the helper reads eagerly and no test produces more than a few batches.
status: wont-fix
---
