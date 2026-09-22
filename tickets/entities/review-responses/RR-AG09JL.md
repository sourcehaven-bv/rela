---
id: RR-AG09JL
type: review-response
title: 'Refusal path was silent: only the override branch was instrumented'
finding: |-
    SelectCommandAuthorizer wired an onOverride callback for the path that GRANTS (override taken on a network bind) but returned denyAuthorizer{} with no callback and no log. The asymmetry is the tell: the branch that was instrumented is the proud one, and the branch that silently changes behavior for an existing deployment was not.

    An operator upgrading a `--bind 0.0.0.0` server with configured commands: gets buttons vanishing from the UI (resolveCommands filters them), /api/command/ 403ing with a deliberately coarse body, and ZERO startup output explaining why. The generic non-loopback warning at cmd/rela-server/main.go:556 fires but says nothing about commands. The upgrade note in docs/server-security.md was the only signal, and nobody reads release notes for a patch bump.
severity: significant
resolution: |-
    Added a symmetric refusal hook. Rather than a fifth positional parameter to a security-critical function (which the same review separately flagged as transposition-prone), the two callbacks are grouped in a CommandAuthNotifier struct with OnOverride and OnRefuse fields; the godoc states why both are needed. cmd/rela-server supplies warnCommandsRefused, which names the remedy (add an acl.yaml with a permission: per command, or pass --allow-unauthenticated-commands) and links docs/server-security.md.

    It fires unconditionally rather than only when commands: are configured, because the authorizer is selected before the config loads; the cost is telling a project with no commands that an unused surface is closed. TestSelectCommandAuthorizer gained a wantOnRefuse column asserting the hook fires on exactly the network-no-policy-no-override path and nowhere else.
status: addressed
---
