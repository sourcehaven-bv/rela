---
id: RR-W983COM
type: review-response
title: 'Design: --token-secret must not read a script-grantable secret'
finding: 'Plan step 3 kept --token-secret reading any global secret as in #2156. A credential stored under an ordinary name can be granted to a script through capabilities.secrets, and a script with an HTTP capability could send it anywhere. The generic protection only works if the credential name is recognizable.'
severity: significant
resolution: Plan now requires NAME to start with remote_credential.; ScriptReadable refuses that prefix and token_key, so a credential --token-secret can read is never grantable to a script.
status: addressed
---
