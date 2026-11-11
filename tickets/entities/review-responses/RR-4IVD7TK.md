---
id: RR-4IVD7TK
type: review-response
title: Push routes the body by local kind while the pull routes by remote name
finding: A mapping of the local body to a remote property was read from the property on pull but written to the remote body on push, reverting local edits.
severity: significant
resolution: FieldMapping.toContent is the one routing rule for pull and push. TestPushBody_RoutesContentLikeThePull.
status: addressed
---
