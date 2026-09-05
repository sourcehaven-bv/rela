// Startup lint for the mail capability gate (TKT-JVHSOZ).
//
// mail.send became capability-gated with no transition period — the gate
// defaults closed and backwards compatibility was explicitly waived. That is
// the right call for an exfiltration primitive, but it means an action script
// that has been mailing a digest every night starts failing on the next
// restart, and nothing at the moment of upgrade says so.
//
// The runtime denial names the fix, but only reaches whoever triggers the
// action, whenever that happens; a nightly digest may not be triggered for a
// day, and a script that ignores mail.send's return value (which it may — the
// binding returns an error, it does not raise) reports nothing at all. So the
// upgrade is announced at BOOT instead, where an operator is already watching
// the log.
package dataentry

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// mailSendNeedle is what the scan looks for in a script body.
//
// A substring, not a parse. See [collectUngatedMailActions] for what that
// costs and why it is the right trade here.
const mailSendNeedle = "mail.send"

// scriptReader loads an action script's source. Declared here rather than
// taking script.ReadActionScript directly so the scan is testable without a
// project on disk (CLAUDE.md: interfaces at the call site).
type scriptReader func(projectRoot, scriptPath string) (string, error)

// collectUngatedMailActions names the actions whose script calls mail.send
// without a `mail:` capability grant.
//
// # This is a HINT, not the control
//
// It is a substring scan over the script source, so it is wrong in both
// directions and is designed to be wrong in the harmless one:
//
//   - It OVER-reports. `-- we used to mail.send here` warns. That costs an
//     operator one glance at a config file.
//   - It UNDER-reports. `local m = mail; m.send{...}`, or a call built by
//     string concatenation, is invisible to it. That costs nothing, because
//     the runtime gate is exact and is what actually denies the send. A missed
//     hint means an operator learns from the denial instead of from the log.
//
// Writing a real Lua parse to close the second gap would be building a second,
// weaker enforcement mechanism beside the working one — and the moment it looks
// authoritative, someone will rely on its silence. A warning that is honest
// about being a heuristic is more useful than one that invites false
// confidence, which is why the log line says "appears to call".
//
// Actions with no `script:` (set-only) are skipped: there is no body to scan.
// A script that cannot be read is skipped silently — startup already failed on
// a missing script by the time this runs, and a lint must not be the thing that
// takes an app down.
func collectUngatedMailActions(
	actions map[string]dataentryconfig.Action, projectRoot string, read scriptReader,
) []string {
	var ungated []string
	for id, action := range actions {
		if action.Script == "" || action.Capabilities.Mail {
			continue
		}
		body, err := read(projectRoot, action.Script)
		if err != nil {
			continue
		}
		if strings.Contains(body, mailSendNeedle) {
			ungated = append(ungated, fmt.Sprintf("%s (%s)", id, action.Script))
		}
	}
	// Sorted so the log line is stable across restarts: map iteration order
	// would otherwise make an unchanged config look like it had changed.
	sort.Strings(ungated)
	return ungated
}

// warnUngatedMailActions logs the startup warning, following the shape
// warnUngatedMembership uses in internal/appbuild: say what is wrong, then name
// the concrete fix and where it is documented.
//
// ONE line listing every affected action rather than one line each — an
// operator upgrading a project with a dozen mailing actions needs a list to
// work through, not a dozen separate alarms about the same change.
func warnUngatedMailActions(
	actions map[string]dataentryconfig.Action, projectRoot string, read scriptReader,
) {
	ungated := collectUngatedMailActions(actions, projectRoot, read)
	if len(ungated) == 0 {
		return
	}
	slog.Warn("data-entry: action script appears to call mail.send without the `mail` capability — "+
		"the send will be DENIED at runtime",
		"actions", strings.Join(ungated, ", "),
		"fix", "add `mail: true` to the action's `capabilities:` block in data-entry.yaml",
		"docs", "docs/lua-scripting.md")
}

// warnUngatedMailActionsFromDisk is the production wiring: the same scan over
// the project's actions/ directory.
func warnUngatedMailActionsFromDisk(actions map[string]dataentryconfig.Action, projectRoot string) {
	warnUngatedMailActions(actions, projectRoot, script.ReadActionScript)
}
