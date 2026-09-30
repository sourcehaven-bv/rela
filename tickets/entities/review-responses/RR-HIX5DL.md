---
id: RR-HIX5DL
type: review-response
title: Archguard missed import aliases, bare identifiers and consumer call sites
finding: recreateEntryPoints only matched the literal selector entitymanager.X and bare calls. An aliased or dot import, a function value, or a new consumer calling recreator.RecreateEntity through an existing wiring escaped the allowlist.
severity: significant
resolution: The matcher resolves the entitymanager import name from file.Imports, matches bare names in the package or under a dot import (declaration names excepted), and pins every x.RecreateEntity selector. history_restore.go and cli/restore.go are now allowlisted call sites. Table cases cover each form.
status: addressed
---
