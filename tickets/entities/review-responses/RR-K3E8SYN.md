---
id: RR-K3E8SYN
type: review-response
title: An operator assertion that carries an org_id reaches tenant data
finding: ServeHTTP refused only an empty org_id and ignored principal_type. An operator assertion with an org would act inside that tenant.
severity: significant
resolution: ServeHTTP refuses principal_type operator on every non-API path with 403 unknown_tenant before Acquire and the status endpoint. TestTenantAPI_OperatorRefusedElsewhere covers an operator with an org.
status: addressed
---
