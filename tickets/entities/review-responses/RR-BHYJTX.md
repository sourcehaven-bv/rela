---
id: RR-BHYJTX
type: review-response
title: Desktop token store skips places trust
finding: Keychain tokens keyed by document id bypass the places trust check; a crafted document with a copied id gets the refresh token.
severity: critical
resolution: 'Plan R3: reuse keychainSecrets with a token/ prefix so places apply; separate items within maxSecretBytes; untrusted-place test.'
status: addressed
---
