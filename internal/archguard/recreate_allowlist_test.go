package archguard

// recreateAllowlist pins every entry point to entitymanager.RecreateEntity
// (see recreateEntryPoints), per repo-relative file. Each is history restore.
var recreateAllowlist = map[string]allowed{
	"internal/entitymanager/recreate.go": {1, "Recreator adapts the package function to a method"},
	"internal/dataentry/app.go":          {1, "wires the Recreator for the history restore handler"},
	"internal/dataentry/history_restore.go": {1, "restoreRecreate, the HTTP history restore of a deleted " +
		"face; it gates every restored property through validateFieldWrite first"},
	"internal/cli/cli_wiring.go": {1, "wires the Recreator for `rela restore`"},
	"internal/cli/restore.go": {1, "`rela restore` of a deleted face; an operator-shell command with " +
		"no field gate, the same trust boundary as every other CLI write"},
}
