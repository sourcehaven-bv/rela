//go:build postgres

package main

// tenantModeBackend reports whether this build can serve tenant mode. Only
// the postgres store is addressed by a DSN, so only it can open one store per
// tenant.
const tenantModeBackend = true
