package metamodel

import (
	"fmt"
	"path"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// JobRetry is a background automation job's retry intent (TKT-2Q4UFI). It
// names the intent only; attempts and backoff belong to internal/jobs.
type JobRetry string

const (
	// JobRetryDefault is the zero value and means JobRetryBounded.
	JobRetryDefault    JobRetry = ""
	JobRetryNever      JobRetry = "never"
	JobRetryBounded    JobRetry = "bounded"
	JobRetryPersistent JobRetry = "persistent"
)

// UnmarshalYAML accepts only the three intents.
func (r *JobRetry) UnmarshalYAML(unmarshal func(any) error) error {
	var raw string
	if err := unmarshal(&raw); err != nil {
		return err
	}
	switch v := JobRetry(strings.ToLower(strings.TrimSpace(raw))); v {
	case JobRetryDefault, JobRetryNever, JobRetryBounded, JobRetryPersistent:
		*r = v
		return nil
	default:
		return fmt.Errorf("invalid retry value %q (want %q, %q or %q)",
			raw, JobRetryNever, JobRetryBounded, JobRetryPersistent)
	}
}

// validateBackgroundActions refuses background actions whose job could not
// be resolved back to exactly one action, or whose keys do not combine.
//
// The job handler finds its action again by automation name and lua_file,
// so both must identify one action. An empty or repeated name would let a
// job run another automation's script, identity or capabilities.
//
// It also checks `on.updated`, which only background actions brought in.
func validateBackgroundActions(m *Metamodel) []string {
	var errs []string
	names := make(map[string]int, len(m.Automations))
	for _, auto := range m.Automations {
		names[auto.Name]++
	}
	for _, auto := range m.Automations {
		errs = append(errs, updatedTriggerErrors(auto)...)
		seen := map[string]bool{}
		for i, a := range auto.Do {
			where := fmt.Sprintf("automation %q action %d", auto.Name, i+1)
			if !a.Background {
				if a.RunAs != "" || a.Retry != JobRetryDefault {
					errs = append(errs, where+": `run_as` and `retry` apply only with `background: true`")
				}
				if len(a.Capabilities.Tokens) > 0 {
					// A token refresh is slow network I/O under a lock, and a
					// synchronous action runs inside the save (TKT-01KZSO).
					errs = append(errs, where+": `capabilities.tokens` applies only with `background: true`")
				}
				continue
			}
			errs = append(errs, backgroundActionErrors(where, a)...)
			if strings.TrimSpace(auto.Name) == "" {
				errs = append(errs, where+": a background action needs a named automation")
			} else if names[auto.Name] > 1 {
				errs = append(errs, where+": a background action needs a unique automation name")
			}
			file := path.Clean(a.LuaFile)
			if seen[file] {
				errs = append(errs, fmt.Sprintf("%s: lua_file %q runs in the background twice", where, a.LuaFile))
			}
			seen[file] = true
		}
	}
	return errs
}

// updatedTriggerErrors refuses `on.updated` beside the property-change keys,
// which would leave unclear which one decides, and beside a foreground
// script. That script's own write is an update, which fires `on.updated`
// again in a nested write that no cascade depth bounds; a background job is
// bounded by its self-suppression and hop limit.
func updatedTriggerErrors(auto AutomationDef) []string {
	if !auto.On.Updated {
		return nil
	}
	var errs []string
	if auto.On.Property != "" || auto.On.Becomes != "" || auto.On.From != "" {
		errs = append(errs, fmt.Sprintf(
			"automation %q: `on.updated` fires on any change; drop it or `on.property`/`becomes`/`from`", auto.Name))
	}
	for i, a := range auto.Do {
		if !a.Background && (a.Lua != "" || a.LuaFile != "") {
			errs = append(errs, fmt.Sprintf(
				"automation %q action %d: a script under `on.updated` must be `background: true`; "+
					"its own write would trigger it again", auto.Name, i+1))
		}
	}
	return errs
}

func backgroundActionErrors(where string, a AutomationAction) []string {
	var errs []string
	if a.LuaFile == "" {
		errs = append(errs, where+": `background: true` needs `lua_file`")
	}
	if a.Lua != "" || a.Set != "" || a.CreateRelation != nil || a.CreateEntity != nil {
		errs = append(errs, where+": a background action may hold only `lua_file`")
	}
	if a.AllowACLBypass.Enabled() {
		errs = append(errs, where+": `allow_acl_bypass` is not supported on a background action")
	}
	if a.RunAs != "" {
		if err := principal.ValidateRunAs(a.RunAs, isAutomationIdentity); err != nil {
			errs = append(errs, where+": "+err.Error())
		}
	}
	return errs
}

// isAutomationIdentity is the one `system:` name a background action may run
// as: the identity it would have without `run_as`. Other `system:` names
// belong to rela's own jobs.
func isAutomationIdentity(s string) bool { return s == principal.UserAutomation }
