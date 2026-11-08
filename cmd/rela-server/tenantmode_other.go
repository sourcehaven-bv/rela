//go:build !postgres

package main

// tenantModeBackend reports whether this build can serve tenant mode. The
// filesystem, memory and sqlite stores keep their data in the project
// directory, so every tenant would read the same data.
const tenantModeBackend = false
