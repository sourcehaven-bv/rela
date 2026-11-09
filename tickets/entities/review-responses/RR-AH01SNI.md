---
id: RR-AH01SNI
type: review-response
title: The DSN-leak assertions tested selection errors only
finding: Selection errors never touch a DSN. The real leak path is the connect error wrapped by openProject.
severity: minor
resolution: TestTenantOpen_ErrorNamesNoPassword sets a base DSN with a password on an unreachable host and asserts the open error names the org but not the password.
status: addressed
---
