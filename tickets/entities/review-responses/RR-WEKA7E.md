---
id: RR-WEKA7E
type: review-response
title: mail resolvePassword swallowed the new corrupt-credential error and reported the wrong cause
finding: |-
    internal/mail/config.go:241 was `if sec, err := secrets.Load(...); err == nil`, discarding every error identically. This change made that pre-existing looseness materially worse by adding a new, likelier, harder-to-diagnose error to the set it swallows.

    With a malformed systemd credential: Load returns a parse error, it is discarded, resolvePassword falls through to os.Getenv(PasswordVar) — reintroducing one layer up the very fallback the credential path deliberately refuses (TestLoad_InvalidYAMLInCredentialDoesNotFallBack) — and if PasswordVar is unset, smtp.go:78 reports "username is set but the environment variable named by password_env is empty or unset".

    That message is now actively false: the credential is present, password_env is irrelevant, and the real fault is a YAML syntax error inside a TPM-encrypted blob the operator cannot easily inspect.
severity: significant
status: addressed
---

## Resolution

`resolvePassword` now distinguishes the three cases, matching what
`internal/lua/context.go` already did:

```go
sec, err := secrets.Load(c.relaDir, "")
switch {
case errors.Is(err, secrets.ErrNotFound):
    // ordinary password_env deployment — silent
case err != nil:
    slog.Warn("mail: secrets source unreadable, falling back to password_env", "error", err)
default:
    if v := sec[SecretKey]; v != "" { return v }
}
```

Kept the fallback rather than returning an error: refusing to send mail over a
secrets-file syntax error is worse than trying the other source. But the real
fault is now on the record instead of surfacing as a misleading message about a
variable that has nothing to do with it.

Two tests, in a package that previously had none for this interaction:
- `TestConfig_MalformedSecretsFallsBackAndWarns` — asserts the fallback still
works, that the warning is emitted, and that it does not echo the credential.
- `TestConfig_AbsentSecretsIsSilent` — the counterpart: no secrets file at all
is the ordinary case and must stay quiet, so the warning keeps its signal.
