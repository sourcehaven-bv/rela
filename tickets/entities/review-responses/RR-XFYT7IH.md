---
id: RR-XFYT7IH
type: review-response
title: Explanation is overloaded for branches
finding: Parallel, Branch and Arrived mean something at one level only; place() evaluates guards for arrived branches and drops them.
severity: nit
reason: One recursive type keeps waitingOf and addDetail shared between the run and its branches. Arrived branches are rare and their guard reads are bounded.
status: wont-fix
---
