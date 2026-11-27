package triggers

import (
	"fmt"
	"path"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// Check reports every rule that the schema cannot honor as written. These
// checks ran in the schema loader while the rules lived in schema.yaml
// (TKT-GWOBH2U moved them here). Each is a load error, because a rule that
// would never fire, or would write what no write path allows, does not mean
// what its author wrote.
//
// [New] runs the checks that need the compiled engine (conditions, file
// properties, notify paths); Check covers the rest.
func Check(meta *metamodel.Metamodel, rules []Rule) []string {
	errs := make([]string, 0, len(rules))
	for _, r := range rules {
		errs = append(errs, firingErrors(r)...)
		errs = append(errs, managedReferenceErrors(r)...)
		errs = append(errs, singleTypeErrors(meta, r)...)
		errs = append(errs, faceErrors(meta, r)...)
		errs = append(errs, copyErrors(meta, r)...)
		errs = append(errs, updatedTriggerErrors(r)...)
		errs = append(errs, deletedTriggerErrors(r)...)
		errs = append(errs, relationCheckErrors(r)...)
		errs = append(errs, backgroundErrors(r)...)
		errs = append(errs, pileErrors(meta, r)...)
		errs = append(errs, externalRefErrors(meta, r)...)
		errs = append(errs, unsetSchemaErrors(meta, r)...)
	}
	return errs
}

func ruleWhere(r Rule) string { return fmt.Sprintf("trigger %q", r.Name) }

// firingErrors refuses a rule that can never do anything. A rule needs an
// event key to fire on, and an output or a check to act with. A check
// severity must be one the engine knows: any other value would quietly
// count as a warning.
func firingErrors(r Rule) []string {
	var errs []string
	on := r.On
	if !on.Created && !on.Updated && !on.Deleted && on.Property == "" &&
		on.RelationCreated == "" && on.RelationRemoved == "" && len(on.Copy) == 0 {

		errs = append(errs, ruleWhere(r)+": `on` needs an event key (created, updated, deleted, property, "+
			"relation_created, relation_removed or copy); the trigger would never fire")
	}
	// deletedTriggerErrors says what an `on.deleted` rule needs instead.
	if len(r.Do) == 0 && len(r.Validate) == 0 && !on.Deleted {
		errs = append(errs, ruleWhere(r)+": `outputs` is empty; the trigger would do nothing")
	}
	for i, v := range r.Validate {
		switch v.Severity {
		case "", SeverityWarning, SeverityError:
		default:
			errs = append(errs, fmt.Sprintf("%s: check %d: severity %q is not %s or %s",
				ruleWhere(r), i+1, v.Severity, SeverityWarning, SeverityError))
		}
	}
	return errs
}

// managedReferenceErrors refuses a rule that names a type or relation rela
// manages (TKT-BC5G8RP): a trigger that fired on, created or linked one would
// act on rela's own records with the operator's rules.
func managedReferenceErrors(r Rule) []string {
	var errs []string
	where := ruleWhere(r)
	ref := func(at, name string) {
		if metamodel.IsReservedName(name) {
			errs = append(errs, fmt.Sprintf("%s: %s: %q is managed by rela and cannot be named here", where, at, name))
		}
	}
	for _, t := range r.On.Entity {
		ref("on.entity", t)
	}
	ref("on.relation_created", r.On.RelationCreated)
	ref("on.relation_removed", r.On.RelationRemoved)
	for _, a := range r.Do {
		if a.CreateRelation != nil {
			ref("relate.type", a.CreateRelation.Relation)
		}
		if a.CreateEntity != nil {
			ref("create.type", a.CreateEntity.Type)
			ref("create.relate.type", a.CreateEntity.Relation)
		}
	}
	return errs
}

// updatedTriggerErrors checks an `on.updated` rule. It fires on any change,
// so a property scope contradicts it, and a script that ran inline would
// fire it again with its own write.
func updatedTriggerErrors(r Rule) []string {
	if !r.On.Updated {
		return nil
	}
	var errs []string
	if r.On.Property != "" || r.On.Becomes != "" || r.On.From != "" {
		errs = append(errs, ruleWhere(r)+": `on.updated` fires on any change; drop it or `on.property`/`becomes`/`from`")
	}
	for i, a := range r.Do {
		if !a.Background && (a.Lua != "" || a.LuaFile != "") {
			errs = append(errs, fmt.Sprintf("%s action %d: a script under `on.updated` must be `background: true`; "+
				"its own write would trigger it again", ruleWhere(r), i+1))
		}
	}
	return errs
}

// relationCheckErrors checks the `check` outputs of a relation rule. A
// relation write returns no warnings, so only a check of severity error,
// which refuses the write, has an effect. A warning check, or one whose
// `where` does not parse, would be silent.
func relationCheckErrors(r Rule) []string {
	if r.On.RelationCreated == "" && r.On.RelationRemoved == "" {
		return nil
	}
	var errs []string
	for i, v := range r.Validate {
		if v.GetSeverity() != SeverityError {
			errs = append(errs, fmt.Sprintf("%s: check %d: a relation trigger cannot show a warning; "+
				"use `severity: error` to refuse the relation write", ruleWhere(r), i+1))
		}
		if _, err := filter.Parse(v.Check); err != nil {
			errs = append(errs, fmt.Sprintf("%s: check %d: invalid `where` clause %q: %v", ruleWhere(r), i+1, v.Check, err))
		}
	}
	return errs
}

// deletedTriggerErrors checks an `on.deleted` rule. The delete has
// committed when it fires, so only background scripts can act on it.
func deletedTriggerErrors(r Rule) []string {
	on := r.On
	if !on.Deleted {
		return nil
	}
	var errs []string
	if on.Created || on.Updated || on.Property != "" || on.Becomes != "" || on.From != "" ||
		on.RelationCreated != "" || on.RelationRemoved != "" {

		errs = append(errs, ruleWhere(r)+": `on.deleted` cannot be combined with another trigger key")
	}
	if len(r.Validate) > 0 {
		errs = append(errs, ruleWhere(r)+": a `check` output does not apply to `on.deleted`; there is no save to warn about")
	}
	if len(r.Do) == 0 {
		errs = append(errs, ruleWhere(r)+": `on.deleted` needs a `background: true` script")
	}
	for i, a := range r.Do {
		if !a.Background {
			errs = append(errs, fmt.Sprintf("%s action %d: an `on.deleted` output must be a `background: true` script; "+
				"the delete has committed when it fires", ruleWhere(r), i+1))
		}
	}
	return errs
}

// backgroundErrors checks the background-only script keys. A queued job
// finds its action again by (trigger name, file), so one rule may run a file
// in the background only once.
func backgroundErrors(r Rule) []string {
	var errs []string
	seen := map[string]bool{}
	for i, a := range r.Do {
		where := fmt.Sprintf("%s action %d", ruleWhere(r), i+1)
		if !a.Background {
			if a.RunAs != "" || a.Retry != metamodel.JobRetryDefault {
				errs = append(errs, where+": `run_as` and `retry` apply only with `background: true`")
			}
			if len(a.Capabilities.Tokens) > 0 {
				errs = append(errs, where+": `capabilities.tokens` applies only with `background: true`")
			}
			continue
		}
		if a.LuaFile == "" {
			errs = append(errs, where+": `background: true` needs `file:`")
		}
		if a.AllowACLBypass.Enabled() {
			errs = append(errs, where+": `allow_acl_bypass` is not supported on a background script")
		}
		if a.RunAs != "" {
			if err := principal.ValidateRunAs(a.RunAs, isTriggerIdentity); err != nil {
				errs = append(errs, where+": "+err.Error())
			}
		}
		file := path.Clean(a.LuaFile)
		if seen[file] {
			errs = append(errs, fmt.Sprintf("%s: file %q runs in the background twice", where, a.LuaFile))
		}
		seen[file] = true
	}
	return errs
}

func isTriggerIdentity(s string) bool { return s == principal.UserAutomation }
