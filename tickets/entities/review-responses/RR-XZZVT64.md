---
id: RR-XZZVT64
type: review-response
title: Case rules differ between refusal and remote config
finding: IsRemoteCredentialEnv ignores case; ValidateDeclared accepts upper case only.
severity: nit
resolution: Comment in ValidateDeclared explains upper case only is safe because lower case is refused elsewhere.
status: addressed
---
