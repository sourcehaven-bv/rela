---
id: RR-FK771VE
type: review-response
title: withheld skip in resolveSectionCreate is unexplained
finding: The withheld condition in resolveSectionCreate is an optimization that no test can observe; a reader could take it for index alignment.
severity: minor
resolution: Added a comment that it only saves the create-target ACL work.
status: addressed
---
