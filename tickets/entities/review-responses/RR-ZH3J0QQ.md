---
id: RR-ZH3J0QQ
type: review-response
title: Peer keys from shared project secrets are valid in every tenant
finding: After the rebase onto develop (TKT-H91N91L), buildApp wired peer keys from the project's .rela/secrets.yaml into every tenant app. A request with a valid org assertion in X-Auth-Assertion and a peer key in Authorization reached that tenant and was stamped peer:<name>. All tenants share the secrets, so one peer key was a credential in every tenant.
severity: significant
resolution: appIdentity gained a peerKeys flag. Single-tenant mode sets it; tenant apps leave it false, so they get no peer keys and a peer key in Authorization is ignored. TestTenantMode_IgnoresPeerKeys pins it (fails with 401 when keys are wired). The postgres and server-security guides and TKT-D6MF3SW document it.
status: addressed
---
