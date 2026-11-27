//go:build sqlite

package entitymanager_test

func init() {
	relTriggerBackends = append(relTriggerBackends, relTriggerBackend{name: "sqlite", open: openConcSQLiteStore})
}
