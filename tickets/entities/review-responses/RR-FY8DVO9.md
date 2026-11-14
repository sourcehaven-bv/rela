---
id: RR-FY8DVO9
type: review-response
title: Env prefix constant has two names
finding: httpremote.CredentialEnvPrefix aliased secrets.RemoteCredentialEnvPrefix.
severity: minor
resolution: Dropped httpremote.CredentialEnvPrefix; callers use secrets.RemoteCredentialEnvPrefix.
status: addressed
---
