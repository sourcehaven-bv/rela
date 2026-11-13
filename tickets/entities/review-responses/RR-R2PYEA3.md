---
id: RR-R2PYEA3
type: review-response
title: Commands inherit REMOTE_CREDENTIAL_ variables
finding: cmdexec.Environ passed REMOTE_CREDENTIAL_* to data-entry commands, transforms, attachment scanners and rela graph, so a command could send the credential anywhere.
severity: significant
resolution: cmdexec.isWithheld withholds every secrets.IsRemoteCredentialEnv name; env_test covers upper and lower case and the Run path.
status: addressed
---
