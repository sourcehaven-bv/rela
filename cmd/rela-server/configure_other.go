//go:build postgres || memorybackend || sqlite

package main

// configEditingBackend reports whether this build can serve
// --config-editing. The sqlite store holds an exclusive lock a rebuild
// cannot take while the old store is open, and postgres nodes would each
// switch on their own.
const configEditingBackend = false
