---
id: RR-96KTUE
type: review-response
title: writesAsIdentity depends on a hand-kept list of read-only commands
finding: "readOnlyCommands (internal/cli/identity.go:149-154) is a third string list, next to requiresProject and actsAsIdentity, keyed on one or two command tokens. A new read-only command prints the \"acting as\" notice without reason. A future write subcommand under a family listed as read-only (acl, attachments, secrets, history) does not print it. The notice also ignores --quiet.\tA future `rela acl fix` or `rela attachments prune` writes as a run-as identity and prints no notice. Nothing fails, so nobody notices.\tMark read-only commands on their kong structs, for example with a `readOnly()` marker method, and check ktx.Selected().Target, or have commands declare what they do. Add a test that walks every kong leaf and requires each to be classified. Decide whether --quiet suppresses the notice, and document the choice."
severity: minor
resolution: readOnlyCommands replaced by identityWrites, which classifies every command that acts as an identity by full command path. TestIdentityWrites_ClassifiesEveryCommand walks the kong tree and fails on an unclassified command. --quiet now suppresses the notice; documented.
reason: readOnlyCommands replaced by identityWrites, which classifies every command that acts as an identity by full command path. TestIdentityWrites_ClassifiesEveryCommand walks the kong tree and fails on an unclassified command. --quiet now suppresses the notice; documented.
status: addressed
---
