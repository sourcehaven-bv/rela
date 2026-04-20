// Package userstate owns per-user, per-repo state that must not be
// synced with the repo tree.
//
// Files managed here include the age identity, the encryption
// last-seen-version marker, rendered-document caches, UI state, user
// defaults, palette overrides, and scheduler execution state. Each
// piece is per-user (lives in the OS-native user-config directory)
// and per-repo (scoped by a stable UUIDv4 fingerprint baked into
// .rela/repo-id).
//
// # Layout
//
// Base directory is resolved cross-platform via os.UserConfigDir():
//
//   - Linux/BSD: $XDG_CONFIG_HOME/rela/repos/<repo-id>/
//     (default: ~/.config/rela/repos/<repo-id>/)
//   - macOS:     ~/Library/Application Support/rela/repos/<repo-id>/
//   - Windows:   %AppData%\rela\repos\<repo-id>\
//
// The $RELA_USER_STATE_DIR environment variable overrides the base
// directory for power-user layouts and tests. The override must be
// absolute and must not point inside the project tree; pointing at a
// known cloud-sync directory (Dropbox, iCloud, OneDrive) logs a
// warning but is permitted.
//
// # Fingerprint resolution
//
// The per-repo fingerprint is a UUIDv4 stored in <project>/.rela/repo-id.
// Cleartext repos generate one on demand; encrypted repos reuse the
// one baked into recipients.age (Keyring.RepoID). Opening an encrypted
// repo whose .rela/repo-id disagrees with the keyring raises
// ErrRepoIDMismatch — the tell-tale signature of a copied-in .rela/
// directory from another project.
//
// # Concurrency
//
// Independent-key writes (ui-state.json, palette.yaml,
// scheduler-state.json) are atomic but last-writer-wins on
// concurrent writers. Security-critical compound operations that
// read a value and write a derived one — specifically the encryption
// last-seen-version and reseal sentinel — must take the Lock method
// on FSService to serialize across processes; see usage in
// internal/encryption/localstate.go and reseal_sentinel.go.
//
// # Permissions
//
// All files are written 0o600, directories 0o700. Windows ACLs do
// not honor POSIX mode bits; the user config directory on Windows
// is user-scoped by default.
//
// On macOS the base rela directory is tagged with
// .metadata_never_index so Spotlight skips it. On Windows the
// directory is marked FILE_ATTRIBUTE_NOT_CONTENT_INDEXED. On Linux
// the equivalent is not applicable (no default-on content indexer).
package userstate
