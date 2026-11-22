---
id: RR-UM9098S
type: review-response
title: "Title link tests repeat each other"
finding: "Both tests assert the same class, and no test covers a row without a link target."
severity: nit
reason: "Merged into one it.each. A row without a target renders a plain span in another template branch; building one needs a list without a detail route, which the mount helper does not support."
status: wont-fix
---
