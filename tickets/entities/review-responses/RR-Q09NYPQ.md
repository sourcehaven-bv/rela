---
id: RR-Q09NYPQ
type: review-response
title: Operator config surfaces can name a remote_credential secret
finding: 'subscription auth.secret, connections.yaml client_secret_secret and the AI key lookup accept a remote_credential. name and would send it to their URL. Same as peer_credential in #2156.'
severity: minor
resolution: 'TKT-RZSTAPZ. Secret-name surfaces (subscriptions auth.secret, connections.yaml, flows webhook secret, capabilities.secrets, Lua filter) already refused remote_credential.* since #2165, each with a test. The env source was open: ai.yaml api_key_env and mail.yaml password_env now refuse REMOTE_CREDENTIAL_* (secrets.IsRemoteCredentialEnv, case-insensitive), and cmdexec withholds those variables from every child process. The AI and mail secret keys are fixed names. Tests per surface; guides updated.'
reason: Operator-authored config, not script reach; a subscription to another rela may need it. Godoc and guides now say no Lua script can read it, not that nothing else can.
status: addressed
---
