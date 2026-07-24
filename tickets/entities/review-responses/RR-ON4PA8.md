---
id: RR-ON4PA8
type: review-response
title: roles_matrix ignored the built-in everyone role
finding: roles_matrix rendered `everyone` as a plain column and never folded its grants into other roles' rows. Since acl appends EveryoneRole to every principal's effective role set, the table understated effective permissions — worst outcome for a security-doc feature. Also the grantsVerb/grantsList replication of acl's unexported logic had already drifted (the everyone fold was what got lost).
severity: significant
resolution: roles_matrix now excludes `everyone` from the columns (namedRoleNames) and OR-folds its grants into every named role's cell (roleGrantsVerb). When everyone grants anything, a footnote is emitted ("✓ cells include grants from the built-in everyone role"). Regression test TestBuild_RolesMatrixEveryoneFolded asserts everyone is not a column, the fold applies, and the footnote appears.
status: addressed
---
