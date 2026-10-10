//go:build !postgres && !memorybackend && !sqlite

package main

// configEditingBackend reports whether this build can serve
// --config-editing. A save rebuilds the store over the new schema, which
// the filesystem store allows while the old one still serves.
const configEditingBackend = true
