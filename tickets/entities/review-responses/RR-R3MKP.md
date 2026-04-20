---
id: RR-R3MKP
type: review-response
title: Fingerprint divergence between .rela/repo-id and Keyring.RepoID() orphans user state on encrypt/decrypt
finding: Plan uses Keyring.RepoID() when encrypted and .rela/repo-id when cleartext. These are two different UUIDs generated at different times. rela keys init moves state to <base>/rela/repos/<RepoID>/; rela keys decrypt rolls it back; a second keys init makes a third dir. User loses defaults, palette, ui-state, last_seen_version. last_seen_version specifically is security-critical (rollback anchor) — tying it to cleartext .rela/repo-id is nonsensical and dangerous if the two IDs ever diverge.
severity: critical
resolution: 'Decision: adopt leverage-opportunity L1 — single canonical fingerprint per repo. Use .rela/repo-id as the sole fingerprint source. Encrypted repos still use Keyring.RepoID() only for encryption-scoped state internally, but the user-state directory root (where key, ui-state, palette, defaults, documents, scheduler-state live) is keyed by .rela/repo-id. On rela keys init, write Keyring.RepoID into .rela/repo-id if empty; fail loudly if they disagree. This makes the user-state directory stable across encrypt/decrypt transitions. Plan updated in Approach section.'
status: addressed
---
